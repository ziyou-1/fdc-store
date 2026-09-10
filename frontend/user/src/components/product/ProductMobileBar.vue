<template>
  <Transition
    enter-active-class="transition duration-300 ease-out"
    enter-from-class="translate-y-full opacity-0"
    enter-to-class="translate-y-0 opacity-100"
    leave-active-class="transition duration-200 ease-in"
    leave-from-class="translate-y-0 opacity-100"
    leave-to-class="translate-y-full opacity-0">
    <div v-if="visible"
      class="lg:hidden fixed bottom-0 left-0 right-0 z-40 bg-card/95 backdrop-blur-xl border-t shadow-2xl theme-safe-bottom">
      <!--
        价格与按钮分成上下两行，而不是挤在同一行。

        挤一行时价格会吃掉约三分之一宽度，375px 视口下每个按钮只剩 70px、
        可用文字宽仅 52px；「Masukkan Keranjang」需要 127px，只显示得下四成，
        中文「加入购物车」也放不下。分行后按钮独占整行宽度，文字才完整。
        参考的淘宝/1688 详情页同样是价格在上、按钮独占一行。
      -->
      <div class="px-3 pb-2.5 pt-2">
        <!-- 价格行：整行给它，长金额（如 25000.00 IDR）也不会被挤断 -->
        <div class="mb-1.5 min-w-0">
          <span v-if="showMemberPrice" class="theme-price-sm text-amber-600 dark:text-amber-300 truncate block">
            {{ memberPriceDisplay }}
          </span>
          <span v-else-if="showSkuPromotionPrice" class="theme-price-sm text-rose-600 dark:text-rose-300 truncate block">
            {{ skuPromotionPriceDisplay }}
          </span>
          <span v-else-if="showSkuPrice" class="theme-price-sm text-primary truncate block">
            {{ skuPriceDisplay }}
          </span>
          <span v-else-if="showProductPromotionPrice" class="theme-price-sm text-rose-600 dark:text-rose-300 truncate block">
            {{ productPromotionPriceDisplay }}
          </span>
          <span v-else class="theme-price-sm text-primary truncate block">
            {{ productPriceDisplay }}
          </span>
        </div>

        <!-- 按钮行：独占整行。不加 truncate——文字宁可换行也不截断 -->
        <div class="flex items-stretch gap-2">
          <Button v-if="requiresLogin" size="lg" class="min-w-0 flex-1 rounded-xl font-bold" @click="$emit('goLogin')">
            {{ t('productDetail.loginToBuy') }}
          </Button>
          <template v-else>
            <Button
              variant="secondary"
              class="min-h-11 h-auto min-w-0 flex-1 basis-0 whitespace-normal rounded-xl px-2 py-1.5 text-xs font-bold leading-tight"
              :disabled="!canPurchase"
              @click="$emit('addToCart')"
            >
              {{ t('productDetail.addToCart') }}
            </Button>
            <Button
              class="min-h-11 h-auto min-w-0 flex-1 basis-0 whitespace-normal rounded-xl px-2 py-1.5 text-xs font-bold leading-tight"
              :disabled="!canPurchase"
              @click="$emit('buyNow')"
            >
              {{ t('productDetail.buyNow') }}
            </Button>
            <Button
              v-if="canShowPointsRedeem"
              variant="outline"
              class="min-h-11 h-auto min-w-0 flex-1 basis-0 whitespace-normal rounded-xl px-2 py-1.5 text-xs font-bold leading-tight"
              :disabled="!canPurchase || redeeming"
              @click="$emit('redeemWithPoints')"
            >
              {{ redeeming ? t('productDetail.redeeming') : t('productDetail.redeemWithPoints') }}
            </Button>
          </template>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'

const { t } = useI18n()

defineProps<{
  visible: boolean
  requiresLogin: boolean
  canPurchase: boolean
  showMemberPrice: boolean
  memberPriceDisplay: string
  showSkuPromotionPrice: boolean
  skuPromotionPriceDisplay: string
  showSkuPrice: boolean
  skuPriceDisplay: string
  showProductPromotionPrice: boolean
  productPromotionPriceDisplay: string
  productPriceDisplay: string
  canShowPointsRedeem?: boolean
  redeeming?: boolean
}>()

defineEmits<{
  addToCart: []
  buyNow: []
  goLogin: []
  redeemWithPoints: []
}>()
</script>
