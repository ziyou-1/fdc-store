<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { notifyError, notifySuccess } from '@/utils/notify'

const { t } = useI18n()

// notifyError 只接受一条消息。拦截器已经弹过的错误带 __notified 标记，
// 这里不再重复弹一次；其余情况优先显示后端的原始错误文案（比如
// 「启用翻译时 DeepL Auth Key 不能为空」），拿不到才回落到兜底文案。
const notifyErrorIfNeeded = (err: unknown, fallback: string) => {
  const known = err as Error & { __notified?: boolean }
  if (known?.__notified) return
  notifyError(known?.message || fallback)
}

const loading = ref(false)
const submitting = ref(false)
const testing = ref(false)

const form = reactive({
  enabled: false,
  // 读接口永远返回空串，这里就是"运营本次输入的新密钥"。
  // 留空提交表示不改密钥——和 Turnstile / SMTP 的表单语义保持一致。
  auth_key: '',
  has_auth_key: false,
})

const loadConfig = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getDeepLSettings()
    const data = res.data?.data as Record<string, unknown> | undefined
    if (data) {
      form.enabled = data.enabled === true
      form.has_auth_key = data.has_auth_key === true
      form.auth_key = ''
    }
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.alerts.loadFailed'))
  } finally {
    loading.value = false
  }
}

const save = async () => {
  submitting.value = true
  try {
    const payload: Record<string, unknown> = { enabled: form.enabled }
    // 只有真填了才提交，避免把已配置的密钥覆盖成空
    if (form.auth_key.trim() !== '') {
      payload.auth_key = form.auth_key.trim()
    }

    const res = await adminAPI.updateDeepLSettings(payload)
    const data = res.data?.data as Record<string, unknown> | undefined
    form.auth_key = ''
    form.has_auth_key = data?.has_auth_key === true || form.has_auth_key
    if (data) form.enabled = data.enabled === true
    notifySuccess(t('admin.settings.alerts.saveSuccess'))
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.alerts.saveFailed'))
  } finally {
    submitting.value = false
  }
}

// 先验后存：翻译在业务侧是 fail-closed 的，密钥填错会让买家发不出客服文本消息。
const testKey = async () => {
  testing.value = true
  try {
    await adminAPI.testDeepLSettings(form.auth_key.trim() ? { auth_key: form.auth_key.trim() } : {})
    notifySuccess(t('admin.settings.translation.testSuccess'))
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.translation.testFailed'))
  } finally {
    testing.value = false
  }
}

// 清除已保存的密钥。空串在补丁语义里表示"保持不变"，
// 所以删除必须走一个显式字段，否则密钥一旦落库就再也删不掉。
const clearKey = async () => {
  if (!window.confirm(t('admin.settings.translation.deepl.clearConfirm'))) return
  submitting.value = true
  try {
    const res = await adminAPI.updateDeepLSettings({ clear_auth_key: true })
    const data = res.data?.data as Record<string, unknown> | undefined
    form.auth_key = ''
    form.has_auth_key = data?.has_auth_key === true
    form.enabled = data?.enabled === true
    notifySuccess(t('admin.settings.translation.deepl.clearSuccess'))
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.alerts.saveFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(loadConfig)

defineExpose({ save, submitting })
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-lg border p-6">
      <h2 class="text-lg font-semibold">{{ t('admin.settings.translation.title') }}</h2>
      <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.translation.subtitle') }}</p>
    </div>

    <div class="rounded-lg border p-6 space-y-4">
      <div class="flex items-center gap-3">
        <Switch v-model="form.enabled" :disabled="loading" />
        <Label class="text-sm font-medium">{{ t('admin.settings.translation.enabled') }}</Label>
      </div>
      <p class="text-xs text-muted-foreground">{{ t('admin.settings.translation.enabledHint') }}</p>
    </div>

    <div class="rounded-lg border p-6 space-y-4">
      <div>
        <h3 class="text-sm font-semibold">{{ t('admin.settings.translation.deepl.title') }}</h3>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.translation.deepl.subtitle') }}</p>
      </div>

      <div class="space-y-1">
        <label class="text-xs font-medium text-muted-foreground">
          {{ t('admin.settings.translation.deepl.authKeyLabel') }}
        </label>
        <Input
          v-model="form.auth_key"
          type="password"
          autocomplete="new-password"
          :disabled="loading"
          :placeholder="t('admin.settings.translation.deepl.authKeyPlaceholder')"
        />
        <p class="text-xs text-muted-foreground">
          {{ form.has_auth_key
            ? t('admin.settings.translation.deepl.authKeyHintKeep')
            : t('admin.settings.translation.deepl.authKeyHintEmpty') }}
        </p>
      </div>

      <div class="flex items-center gap-3">
        <Button variant="outline" size="sm" :disabled="testing || loading" @click="testKey">
          {{ testing ? t('admin.settings.translation.testing') : t('admin.settings.translation.testButton') }}
        </Button>
        <Button
          v-if="form.has_auth_key"
          variant="outline"
          size="sm"
          :disabled="submitting || loading"
          @click="clearKey"
        >
          {{ t('admin.settings.translation.deepl.clearButton') }}
        </Button>
        <span class="text-xs text-muted-foreground">{{ t('admin.settings.translation.testHint') }}</span>
      </div>

      <div class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300">
        {{ t('admin.settings.translation.failClosedWarning') }}
      </div>
    </div>
  </div>
</template>
