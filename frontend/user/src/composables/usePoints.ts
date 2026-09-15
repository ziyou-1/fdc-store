import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { pointsAPI, walletAPI } from '../api'
import type {
    PointsAccountData,
    PointsCheckinStatusData,
    PointsInviteData,
    PointsPublicConfigData,
    PointsRechargeRecordData,
    PointsTransactionData,
} from '../api/types'
import type { PageAlert } from '../utils/alerts'
import { reservePayWindow } from '../utils/payWindow'

const PAGE_SIZE = 20

export function usePoints() {
    const { t } = useI18n()

    const loading = ref(true)
    const checkinLoading = ref(false)
    const recharging = ref(false)
    const channelLoading = ref(false)

    const account = ref<PointsAccountData>({ balance: 0, total_earned: 0, total_spent: 0 })
    const checkinStatus = ref<PointsCheckinStatusData | null>(null)
    const config = ref<PointsPublicConfigData | null>(null)
    const invite = ref<PointsInviteData | null>(null)
    const transactions = ref<PointsTransactionData[]>([])
    const recharges = ref<PointsRechargeRecordData[]>([])
    const channels = ref<any[]>([])
    const walletBalance = ref('0')
    const alert = ref<PageAlert | null>(null)

    const pagination = ref({ page: 1, page_size: PAGE_SIZE, total: 0, total_page: 1 })

    // payWithBalance 为 true 时用钱包余额支付，此时忽略 channelId。
    const rechargeForm = reactive({ amount: '', channelId: 0, remark: '', payWithBalance: false })

    const rechargeEnabled = computed(() => config.value?.recharge_enabled === true)
    const pointsPerUnit = computed(() => config.value?.recharge_points_per_unit ?? 0)
    const checkinEnabled = computed(() => checkinStatus.value?.enabled === true)
    const canCheckin = computed(
        () => checkinEnabled.value && checkinStatus.value?.checked_in === false && !checkinLoading.value,
    )

    const loadAccount = async () => {
        const res = await pointsAPI.account()
        account.value = res.data.data
    }

    const loadWalletBalance = async () => {
        try {
            const res = await walletAPI.account()
            walletBalance.value = String(res.data.data?.balance ?? '0')
        } catch {
            // 钱包余额拉不到不影响主面板，只是余额支付选项无法显示可用额度
        }
    }

    const loadConfig = async () => {
        const res = await pointsAPI.config()
        config.value = res.data.data
    }

    const loadInvite = async () => {
        const res = await pointsAPI.invite()
        invite.value = res.data.data
    }

    const loadCheckinStatus = async () => {
        const res = await pointsAPI.checkinStatus()
        checkinStatus.value = res.data.data
    }

    const loadTransactions = async (page = 1) => {
        loading.value = true
        try {
            const res = await pointsAPI.transactions({ page, page_size: PAGE_SIZE })
            transactions.value = res.data.data ?? []
            const meta = res.data.pagination
            if (meta) pagination.value = meta
        } catch (err: any) {
            alert.value = { level: 'error', message: err?.message || t('personalCenter.points.errors.loadFailed') }
        } finally {
            loading.value = false
        }
    }

    const loadRecharges = async () => {
        try {
            const res = await pointsAPI.recharges({ page: 1, page_size: 5 })
            recharges.value = res.data.data ?? []
        } catch {
            // 充值记录拉不到不影响主面板
        }
    }

    const handleCheckin = async () => {
        if (!canCheckin.value) return
        checkinLoading.value = true
        alert.value = null
        try {
            const res = await pointsAPI.checkin()
            const result = res.data.data
            alert.value = {
                level: 'success',
                message: t('personalCenter.points.checkinSuccess', {
                    points: result.points,
                    days: result.streak_days,
                }),
            }
            // 签到成功后刷新三处；用 allSettled 避免其中一个刷新失败被误判成签到失败
            await Promise.allSettled([loadAccount(), loadCheckinStatus(), loadTransactions(1)])
        } catch (err: any) {
            alert.value = { level: 'error', message: err?.message || t('personalCenter.points.errors.checkinFailed') }
        } finally {
            checkinLoading.value = false
        }
    }

    // 渠道列表按金额拉取（后端按金额过滤可用渠道），金额为空时不请求
    const loadChannels = async () => {
        const amount = rechargeForm.amount.trim()
        if (!amount) {
            channels.value = []
            return
        }
        channelLoading.value = true
        try {
            const res = await walletAPI.getPaymentChannels(amount)
            channels.value = res.data.data ?? []
            // 原选中渠道如果不在新列表里，清空选择，避免提交一个已不可用的渠道
            if (!channels.value.some((item: any) => item.id === rechargeForm.channelId)) {
                rechargeForm.channelId = 0
            }
        } catch {
            channels.value = []
            rechargeForm.channelId = 0
        } finally {
            channelLoading.value = false
        }
    }

    const handleRecharge = async () => {
        alert.value = null
        const amount = rechargeForm.amount.trim()
        if (!amount || Number(amount) <= 0) {
            alert.value = { level: 'warning', message: t('personalCenter.points.errors.invalidAmount') }
            return
        }
        if (!rechargeForm.payWithBalance && rechargeForm.channelId <= 0) {
            alert.value = { level: 'warning', message: t('personalCenter.points.errors.channelRequired') }
            return
        }
        // 余额支付前先在前台挡一道：余额不足直接提示，不必往返一次后端。
        if (rechargeForm.payWithBalance && Number(walletBalance.value) < Number(amount)) {
            alert.value = { level: 'warning', message: t('personalCenter.points.errors.balanceInsufficient') }
            return
        }
        const paidByBalance = rechargeForm.payWithBalance
        recharging.value = true
        // 在线支付才需要新标签；余额支付即时到账、不跳转，不开标签。
        // 在 await 之前同步占位，避免弹窗被拦。
        const payWindow = paidByBalance ? null : reservePayWindow()
        try {
            const payload = paidByBalance
                ? { amount, pay_with_balance: true, remark: rechargeForm.remark.trim() || undefined }
                : { amount, channel_id: rechargeForm.channelId, remark: rechargeForm.remark.trim() || undefined }
            const res = await pointsAPI.recharge(payload)
            const result = res.data.data || {}
            rechargeForm.amount = ''
            rechargeForm.remark = ''
            channels.value = []
            rechargeForm.channelId = 0
            rechargeForm.payWithBalance = false
            await loadRecharges()
            // 钱包余额支付即时到账：刷新积分账户与余额，直接提示成功。
            if (paidByBalance || result.status === 'success' || result.interaction_mode === 'balance') {
                payWindow?.release()
                await Promise.allSettled([loadAccount(), loadWalletBalance()])
                alert.value = {
                    level: 'success',
                    message: t('personalCenter.points.rechargeBalanceSuccess', { points: result.points_amount ?? 0 }),
                }
                return null
            }
            // 在线支付：后端只会给 redirect 或 qr 两种模式
            if (result.redirect_url) {
                if (payWindow) payWindow.navigate(result.redirect_url)
                else window.location.href = result.redirect_url
                return null
            }
            payWindow?.release()
            if (result.qr_code) {
                return result
            }
            alert.value = { level: 'success', message: t('personalCenter.points.rechargeCreated') }
            return null
        } catch (err: any) {
            payWindow?.release()
            alert.value = { level: 'error', message: err?.message || t('personalCenter.points.errors.rechargeFailed') }
            return null
        } finally {
            recharging.value = false
        }
    }

    const changePage = (page: number) => {
        if (page < 1 || page > pagination.value.total_page) return
        void loadTransactions(page)
    }

    const initialize = async () => {
        loading.value = true
        // 账户与签到状态失败不应让整个面板空白，各自兜底
        const results = await Promise.allSettled([
            loadAccount(),
            loadCheckinStatus(),
            loadConfig(),
            loadInvite(),
            loadRecharges(),
            loadWalletBalance(),
        ])
        const failed = results.find((item) => item.status === 'rejected')
        if (failed && failed.status === 'rejected') {
            alert.value = {
                level: 'error',
                message: (failed.reason as Error)?.message || t('personalCenter.points.errors.loadFailed'),
            }
        }
        await loadTransactions(1)
    }

    return {
        loading,
        checkinLoading,
        recharging,
        channelLoading,
        account,
        checkinStatus,
        config,
        invite,
        transactions,
        recharges,
        channels,
        walletBalance,
        alert,
        pagination,
        rechargeForm,
        checkinEnabled,
        rechargeEnabled,
        pointsPerUnit,
        canCheckin,
        handleCheckin,
        handleRecharge,
        loadChannels,
        loadTransactions,
        changePage,
        initialize,
    }
}
