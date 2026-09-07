<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Gift, Loader2 } from 'lucide-vue-next'
import { freeClaimAPI } from '../api'
import type { FreeClaimTodayData } from '../api/types'
import { useLocalized } from '../composables/useProduct'
import { useToast } from '../composables/useToast'
import { useUserAuthStore } from '../stores/userAuth'
import { getImageUrl } from '../utils/image'
import { Button } from '@/components/ui/button'
import SmartImage from './SmartImage.vue'

const { t } = useI18n()
const router = useRouter()
const { toast } = useToast()
const { getLocalizedText, formatPrice } = useLocalized()
const userAuthStore = useUserAuthStore()

const claiming = ref(false)
const today = ref<FreeClaimTodayData | null>(null)

// 站点未开启、今日无可领商品，或商品信息解析不出来时整卡不渲染。
// 首页不该出现一张"暂无"的空卡占位。
const visible = computed(
  () => today.value?.enabled === true && today.value?.available === true && !!today.value?.product,
)
const product = computed(() => today.value?.product ?? null)
const claimed = computed(() => today.value?.claimed === true)
const memberOnly = computed(() => today.value?.member_only === true)
const eligible = computed(() => today.value?.eligible === true)
const isAuthenticated = computed(() => userAuthStore.isAuthenticated)

const productTitle = computed(() => {
  if (!product.value) return ''
  return getLocalizedText(product.value.title) || Object.values(product.value.title)[0] || ''
})
const productLink = computed(() =>
  product.value?.slug ? `/products/${encodeURIComponent(product.value.slug)}` : '',
)

const load = async () => {
  try {
    const res = await freeClaimAPI.today()
    today.value = res.data.data
  } catch {
    // 首页组件失败必须静默：不能因为一个营销卡片把首页搞出报错
    today.value = null
  }
}

const handleClaim = async () => {
  if (!isAuthenticated.value) {
    router.push(`/auth/login?redirect=${encodeURIComponent('/')}`)
    return
  }
  if (!eligible.value) {
    router.push('/me/membership')
    return
  }
  if (claiming.value || claimed.value) return
  claiming.value = true
  try {
    const res = await freeClaimAPI.claim()
    const orderNo = res.data.data?.order_no
    toast.success(t('freeClaim.claimSuccess'))
    // 领取即生成订单，直接送到订单详情看卡密，省一次“我领了但东西在哪”的困惑
    if (orderNo) {
      router.push(`/orders/${encodeURIComponent(orderNo)}`)
      return
    }
    await load()
  } catch (err: any) {
    toast.error(err?.message || t('freeClaim.claimFailed'))
    // 失败可能是"今日已领"这类状态变化，刷新一次让卡片跟上真实状态
    await load()
  } finally {
    claiming.value = false
  }
}

onMounted(load)
</script>

<template>
  <div
    v-if="visible"
    class="flex flex-col gap-5 rounded-2xl border border-primary/20 bg-primary/5 p-5 sm:flex-row sm:items-center sm:gap-6 sm:p-6"
  >
      <RouterLink
        v-if="productLink"
        :to="productLink"
        class="h-24 w-24 shrink-0 overflow-hidden rounded-xl border border-border bg-card sm:h-28 sm:w-28"
      >
        <SmartImage
          :src="getImageUrl(product?.image)"
          :alt="productTitle"
          img-class="h-full w-full object-cover"
        />
      </RouterLink>

      <div class="min-w-0 flex-1">
        <p class="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.18em] text-primary">
          <Gift class="h-3.5 w-3.5" aria-hidden="true" />
          {{ memberOnly ? t('freeClaim.memberEyebrow') : t('freeClaim.eyebrow') }}
        </p>
        <h2 class="mt-1.5 truncate text-lg font-black text-foreground sm:text-xl">
          <RouterLink v-if="productLink" :to="productLink" class="hover:underline">
            {{ productTitle }}
          </RouterLink>
          <span v-else>{{ productTitle }}</span>
        </h2>
        <p class="mt-1 text-sm text-muted-foreground">
          <span v-if="product?.price" class="mr-1 line-through">{{ formatPrice(product.price) }}</span>
          <span class="font-semibold text-primary">{{ t('freeClaim.free') }}</span>
          <span class="ml-2">{{ t('freeClaim.oncePerDay') }}</span>
        </p>
        <p v-if="memberOnly" class="mt-1 text-xs text-muted-foreground">{{ t('freeClaim.memberOnlyHint') }}</p>
      </div>

      <div class="shrink-0">
        <Button v-if="claimed" type="button" variant="outline" disabled class="w-full sm:w-auto">
          {{ t('freeClaim.claimed') }}
        </Button>
        <Button v-else type="button" class="w-full gap-2 sm:w-auto" :disabled="claiming" @click="handleClaim">
          <Loader2 v-if="claiming" class="h-4 w-4 animate-spin" aria-hidden="true" />
          {{
            !isAuthenticated
              ? t('freeClaim.loginToClaim')
              : eligible
                ? t('freeClaim.claimNow')
                : t('freeClaim.becomeMember')
          }}
        </Button>
      </div>
  </div>
</template>
