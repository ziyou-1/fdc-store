<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Gift } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { PointsInviteData } from '../../api/types'
import { copyText } from '../../utils/clipboard'
import { referralInviteURL } from '../../utils/referralInvite'

const props = defineProps<{
  invite: PointsInviteData
}>()

const { t } = useI18n()
const copied = ref<'code' | 'link' | ''>('')
const copyError = ref('')

const inviteLink = () => referralInviteURL(window.location.origin, props.invite.code)

const copy = async (kind: 'code' | 'link') => {
  copyError.value = ''
  const value = kind === 'code' ? props.invite.code : inviteLink()
  try {
    await copyText(value)
    copied.value = kind
  } catch {
    copyError.value = t('personalCenter.points.errors.copyFailed')
  }
}
</script>

<template>
  <div v-if="invite.enabled" class="rounded-2xl border bg-card p-7 shadow-sm">
    <div class="flex flex-col gap-4">
      <div class="flex items-center gap-3.5">
        <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary">
          <Gift class="h-5 w-5" />
        </span>
        <div>
          <h3 class="text-base font-black text-foreground">{{ t('personalCenter.points.referralTitle') }}</h3>
          <p class="mt-1 text-sm text-muted-foreground">
            {{ t('personalCenter.points.referralHint', { register: invite.register_points, order: invite.first_order_points }) }}
          </p>
        </div>
      </div>

      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div class="text-xs text-muted-foreground">{{ t('personalCenter.points.referralCode') }}</div>
          <span class="mt-1 inline-block rounded-lg border border-border bg-muted/30 px-2 py-1 font-mono text-sm text-foreground">
            {{ invite.code }}
          </span>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button type="button" variant="outline" size="sm" @click="copy('code')">
            {{ copied === 'code' ? t('personalCenter.points.referralCopied') : t('personalCenter.points.referralCopyCode') }}
          </Button>
          <Button type="button" variant="outline" size="sm" @click="copy('link')">
            {{ copied === 'link' ? t('personalCenter.points.referralCopied') : t('personalCenter.points.referralCopyLink') }}
          </Button>
        </div>
      </div>
      <p v-if="copyError" class="text-sm text-destructive">{{ copyError }}</p>
    </div>
  </div>
</template>
