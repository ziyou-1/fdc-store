<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminPointsConfig } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { notifySuccess } from '@/utils/notify'

const { t } = useI18n()

const emptyConfig = (): AdminPointsConfig => ({
  checkin_enabled: false,
  checkin_points: 0,
  referral_register_points: 0,
  referral_first_order_points: 0,
  recharge_points_per_unit: 0,
  exchange_enabled: false,
  exchange_daily_limit: 0,
  exchange_sku_daily_limit: 0,
  recharge_enabled: false,
  referral_enabled: false,
  referral_daily_limit: 0,
  referral_first_order_min_amount: '0.00',
})

const loading = ref(true)
const saving = ref(false)
const formError = ref('')
const config = reactive<AdminPointsConfig>(emptyConfig())

const asInt = (value: unknown) => {
  const n = Math.trunc(Number(value))
  return Number.isFinite(n) && n > 0 ? n : 0
}

const asAmount = (value: unknown) => {
  const raw = String(value ?? '').trim()
  if (!raw) return '0.00'
  const n = Number(raw)
  if (!Number.isFinite(n) || n < 0) return ''
  return n.toFixed(2)
}

const applyConfig = (data: Partial<AdminPointsConfig> | undefined) => {
  const next = emptyConfig()
  if (data) {
    next.checkin_enabled = Boolean(data.checkin_enabled)
    next.checkin_points = asInt(data.checkin_points)
    next.referral_register_points = asInt(data.referral_register_points)
    next.referral_first_order_points = asInt(data.referral_first_order_points)
    next.recharge_points_per_unit = asInt(data.recharge_points_per_unit)
    next.exchange_enabled = Boolean(data.exchange_enabled)
    next.exchange_daily_limit = asInt(data.exchange_daily_limit)
    next.exchange_sku_daily_limit = asInt(data.exchange_sku_daily_limit)
    next.recharge_enabled = Boolean(data.recharge_enabled)
    next.referral_enabled = Boolean(data.referral_enabled)
    next.referral_daily_limit = asInt(data.referral_daily_limit)
    next.referral_first_order_min_amount = asAmount(data.referral_first_order_min_amount) || '0.00'
  }
  Object.assign(config, next)
}

const payload = (): AdminPointsConfig => ({
  checkin_enabled: config.checkin_enabled,
  checkin_points: asInt(config.checkin_points),
  referral_register_points: asInt(config.referral_register_points),
  referral_first_order_points: asInt(config.referral_first_order_points),
  recharge_points_per_unit: asInt(config.recharge_points_per_unit),
  exchange_enabled: config.exchange_enabled,
  exchange_daily_limit: asInt(config.exchange_daily_limit),
  exchange_sku_daily_limit: asInt(config.exchange_sku_daily_limit),
  recharge_enabled: config.recharge_enabled,
  referral_enabled: config.referral_enabled,
  referral_daily_limit: asInt(config.referral_daily_limit),
  referral_first_order_min_amount: asAmount(config.referral_first_order_min_amount) || '0.00',
})

const fetchConfig = async () => {
  loading.value = true
  try {
    const response = await adminAPI.getPointsConfig()
    applyConfig(response.data.data)
  } catch {
    // client 已统一提示
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  const data = payload()
  if (data.referral_enabled && data.referral_daily_limit <= 0) {
    formError.value = t('admin.pointsConfig.errors.referralLimitRequired')
    return
  }
  if (data.recharge_enabled && data.recharge_points_per_unit <= 0) {
    formError.value = t('admin.pointsConfig.errors.rechargeRateRequired')
    return
  }
  if (asAmount(config.referral_first_order_min_amount) === '') {
    formError.value = t('admin.pointsConfig.errors.minAmountInvalid')
    return
  }

  formError.value = ''
  saving.value = true
  try {
    const response = await adminAPI.updatePointsConfig(data)
    applyConfig(response.data.data)
    notifySuccess(t('admin.pointsConfig.saveSuccess'))
  } catch {
    // client 已提示
  } finally {
    saving.value = false
  }
}

onMounted(fetchConfig)
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-semibold">{{ t('admin.pointsConfig.title') }}</h1>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.pointsConfig.subtitle') }}</p>
    </div>

    <div class="rounded-xl border border-border bg-card p-6 shadow-sm">
      <div v-if="loading" class="h-48 animate-pulse rounded-lg bg-muted"></div>
      <div v-else class="space-y-8">
        <section class="space-y-4">
          <h2 class="text-base font-semibold">{{ t('admin.pointsConfig.sections.checkin') }}</h2>
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="text-sm font-medium">{{ t('admin.pointsConfig.form.checkinEnabled') }}</p>
              <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.pointsConfig.form.checkinEnabledHint') }}</p>
            </div>
            <Switch v-model="config.checkin_enabled" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.pointsConfig.form.checkinPoints') }}
            </label>
            <Input
              type="number"
              min="0"
              step="1"
              class="sm:max-w-xs"
              :model-value="config.checkin_points"
              @update:model-value="config.checkin_points = asInt($event)"
            />
          </div>
        </section>

        <section class="space-y-4">
          <h2 class="text-base font-semibold">{{ t('admin.pointsConfig.sections.recharge') }}</h2>
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="text-sm font-medium">{{ t('admin.pointsConfig.form.rechargeEnabled') }}</p>
              <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.pointsConfig.form.rechargeEnabledHint') }}</p>
            </div>
            <Switch v-model="config.recharge_enabled" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.pointsConfig.form.rechargePointsPerUnit') }}
            </label>
            <Input
              type="number"
              min="0"
              step="1"
              class="sm:max-w-xs"
              :model-value="config.recharge_points_per_unit"
              @update:model-value="config.recharge_points_per_unit = asInt($event)"
            />
            <p class="mt-1 text-[11px] leading-5 text-muted-foreground">
              {{ t('admin.pointsConfig.form.rechargePointsPerUnitHint') }}
            </p>
          </div>
        </section>

        <section class="space-y-4">
          <h2 class="text-base font-semibold">{{ t('admin.pointsConfig.sections.exchange') }}</h2>
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="text-sm font-medium">{{ t('admin.pointsConfig.form.exchangeEnabled') }}</p>
              <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.pointsConfig.form.exchangeEnabledHint') }}</p>
            </div>
            <Switch v-model="config.exchange_enabled" />
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.pointsConfig.form.exchangeDailyLimit') }}
              </label>
              <Input
                type="number"
                min="0"
                step="1"
                :model-value="config.exchange_daily_limit"
                @update:model-value="config.exchange_daily_limit = asInt($event)"
              />
              <p class="mt-1 text-[11px] leading-5 text-muted-foreground">
                {{ t('admin.pointsConfig.form.limitZeroHint') }}
              </p>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.pointsConfig.form.exchangeSkuDailyLimit') }}
              </label>
              <Input
                type="number"
                min="0"
                step="1"
                :model-value="config.exchange_sku_daily_limit"
                @update:model-value="config.exchange_sku_daily_limit = asInt($event)"
              />
              <p class="mt-1 text-[11px] leading-5 text-muted-foreground">
                {{ t('admin.pointsConfig.form.limitZeroHint') }}
              </p>
            </div>
          </div>
        </section>

        <section class="space-y-4">
          <h2 class="text-base font-semibold">{{ t('admin.pointsConfig.sections.referral') }}</h2>
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="text-sm font-medium">{{ t('admin.pointsConfig.form.referralEnabled') }}</p>
              <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.pointsConfig.form.referralEnabledHint') }}</p>
            </div>
            <Switch v-model="config.referral_enabled" />
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.pointsConfig.form.referralRegisterPoints') }}
              </label>
              <Input
                type="number"
                min="0"
                step="1"
                :model-value="config.referral_register_points"
                @update:model-value="config.referral_register_points = asInt($event)"
              />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.pointsConfig.form.referralFirstOrderPoints') }}
              </label>
              <Input
                type="number"
                min="0"
                step="1"
                :model-value="config.referral_first_order_points"
                @update:model-value="config.referral_first_order_points = asInt($event)"
              />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.pointsConfig.form.referralDailyLimit') }}
              </label>
              <Input
                type="number"
                min="0"
                step="1"
                :model-value="config.referral_daily_limit"
                @update:model-value="config.referral_daily_limit = asInt($event)"
              />
              <p class="mt-1 text-[11px] leading-5 text-muted-foreground">
                {{ t('admin.pointsConfig.form.referralDailyLimitHint') }}
              </p>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.pointsConfig.form.referralFirstOrderMinAmount') }}
              </label>
              <Input
                :model-value="config.referral_first_order_min_amount"
                @update:model-value="config.referral_first_order_min_amount = String($event ?? '')"
              />
              <p class="mt-1 text-[11px] leading-5 text-muted-foreground">
                {{ t('admin.pointsConfig.form.referralFirstOrderMinAmountHint') }}
              </p>
            </div>
          </div>
        </section>

        <p v-if="formError" class="text-sm text-destructive">{{ formError }}</p>
        <Button size="sm" :disabled="saving" @click="handleSave">
          {{ saving ? t('admin.common.submitting') : t('admin.common.save') }}
        </Button>
      </div>
    </div>
  </div>
</template>
