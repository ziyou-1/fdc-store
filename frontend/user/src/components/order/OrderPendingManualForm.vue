<template>
  <div
    v-if="manualFormProducts.length"
    class="rounded-2xl border bg-card text-card-foreground p-6"
  >
    <h2 class="mb-2 text-lg font-bold text-foreground">{{ t('orderDetail.manualFormTitle') }}</h2>
    <p class="mb-2 text-xs text-muted-foreground">{{ t('orderDetail.manualFormTip') }}</p>
    <p class="mb-4 text-xs text-amber-700 dark:text-amber-400">
      {{ t('orderDetail.manualFormSla', { hours: slaHours }) }}
      <span v-if="deadline"> {{ t('orderDetail.manualFormDeadline', { time: deadline }) }}</span>
    </p>
    <CheckoutManualForm
      embedded
      :manual-form-products="manualFormProducts"
      :model-value="modelValue"
      :submit-attempted="submitAttempted"
      :get-manual-field-label="getManualFieldLabel"
      :get-manual-field-placeholder="getManualFieldPlaceholder"
      :manual-field-error="manualFieldError"
      @update:model-value="$emit('update:modelValue', $event)"
    />
    <Button class="mt-4" :disabled="submitting" @click="$emit('submit')">
      {{ submitting ? t('orderDetail.manualFormSubmitting') : t('orderDetail.manualFormSubmit') }}
    </Button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import CheckoutManualForm from '../checkout/CheckoutManualForm.vue'

const { t } = useI18n()

defineProps<{
  manualFormProducts: any[]
  modelValue: Record<string, Record<string, any>>
  submitAttempted: boolean
  submitting: boolean
  slaHours: number
  deadline?: string
  getManualFieldLabel: (field: any) => string
  getManualFieldPlaceholder: (field: any) => string
  manualFieldError: (itemKey: string, fieldKey: string) => string
}>()

defineEmits<{
  (e: 'update:modelValue', value: Record<string, Record<string, any>>): void
  (e: 'submit'): void
}>()
</script>
