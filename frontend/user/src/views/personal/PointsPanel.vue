<template>
  <div class="space-y-6">
    <PointsSummaryCard :account="account" :alert="alert" />

    <PointsCheckinCard
      :status="checkinStatus"
      :loading="checkinLoading"
      :can-checkin="canCheckin"
      @checkin="handleCheckin"
    />

    <PointsReferralCard v-if="invite" :invite="invite" />

    <PointsRechargeForm
      v-if="rechargeEnabled"
      :amount="rechargeForm.amount"
      :channel-id="rechargeForm.channelId"
      :remark="rechargeForm.remark"
      :channels="channels"
      :recharging="recharging"
      :channel-loading="channelLoading"
      :points-per-unit="pointsPerUnit"
      :wallet-balance="walletBalance"
      :pay-with-balance="rechargeForm.payWithBalance"
      @update:amount="rechargeForm.amount = $event"
      @update:channel-id="rechargeForm.channelId = $event"
      @update:remark="rechargeForm.remark = $event"
      @update:pay-with-balance="rechargeForm.payWithBalance = $event"
      @amount-blur="loadChannels"
      @submit="onRecharge"
    />

    <!-- 扫码支付：后端只会返回 redirect 或 qr，redirect 已在 composable 里直接跳转 -->
    <div v-if="qrCode" class="rounded-2xl border bg-card p-7 text-center shadow-sm">
      <h3 class="mb-4 text-base font-black text-foreground">
        {{ t('personalCenter.points.scanToPay') }}
      </h3>
      <img v-if="qrDataUrl" :src="qrDataUrl" :alt="t('personalCenter.points.scanToPay')" class="mx-auto h-48 w-48" />
      <p class="mt-3 break-all text-xs text-muted-foreground">{{ qrCode }}</p>
    </div>

    <PointsTransactionList
      :loading="loading"
      :transactions="transactions"
      :current-page="pagination.page"
      :total-pages="pagination.total_page"
      @refresh="loadTransactions(pagination.page)"
      @change-page="changePage"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import { usePoints } from '../../composables/usePoints'
import PointsSummaryCard from '../../components/points/PointsSummaryCard.vue'
import PointsCheckinCard from '../../components/points/PointsCheckinCard.vue'
import PointsReferralCard from '../../components/points/PointsReferralCard.vue'
import PointsRechargeForm from '../../components/points/PointsRechargeForm.vue'
import PointsTransactionList from '../../components/points/PointsTransactionList.vue'

const { t } = useI18n()

const {
  loading,
  checkinLoading,
  recharging,
  channelLoading,
  account,
  checkinStatus,
  invite,
  transactions,
  channels,
  walletBalance,
  alert,
  pagination,
  rechargeForm,
  rechargeEnabled,
  pointsPerUnit,
  canCheckin,
  handleCheckin,
  handleRecharge,
  loadChannels,
  loadTransactions,
  changePage,
  initialize,
} = usePoints()

const qrCode = ref('')
const qrDataUrl = ref('')

const onRecharge = async () => {
  qrCode.value = ''
  qrDataUrl.value = ''
  const result = await handleRecharge()
  if (result?.qr_code) {
    qrCode.value = result.qr_code
    try {
      qrDataUrl.value = await QRCode.toDataURL(result.qr_code)
    } catch {
      // 二维码渲染失败时仍保留原始串，用户可手动复制
    }
  }
}

onMounted(initialize)
</script>
