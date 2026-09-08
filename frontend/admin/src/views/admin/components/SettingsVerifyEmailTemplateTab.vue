<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { notifyError, notifySuccess } from '@/utils/notify'

const { t } = useI18n()

type LocalizedTemplate = { subject: string; body: string }

const props = defineProps<{ currentLang: string }>()

// 场景与语言都由后端返回，前端不再自己写死一份清单——
// 订单邮件模板正是因为写死了语言列表，新增印尼语时漏掉了它。
const scenes = ref<string[]>([])
const locales = ref<string[]>([])
const variables = ref<string[]>([])
const currentScene = ref<string>('')

const loading = ref(false)
const submitting = ref(false)
const resetting = ref(false)

const form = reactive<{ templates: Record<string, Record<string, LocalizedTemplate>> }>({ templates: {} })

const blank = (): LocalizedTemplate => ({ subject: '', body: '' })

// API 客户端在业务码非 0 时已经弹过 toast 并给错误打了 __notified 标记，
// 这里必须跳过，否则同一个错误会连弹两次。
const notifyErrorIfNeeded = (err: unknown, fallback: string) => {
  const known = err as Error & { __notified?: boolean }
  if (known?.__notified) return
  notifyError(known?.message || fallback)
}

// 取当前场景+语言的模板；后端保证有值，这里兜底避免首次渲染时报 undefined
const cell = (scene: string, locale: string): LocalizedTemplate => {
  let sceneForm = form.templates[scene]
  if (!sceneForm) {
    sceneForm = {}
    form.templates[scene] = sceneForm
  }
  let t = sceneForm[locale]
  if (!t) {
    t = blank()
    sceneForm[locale] = t
  }
  return t
}

const applyResponse = (data: Record<string, unknown>) => {
  scenes.value = (data.scenes as string[]) || []
  locales.value = (data.locales as string[]) || []
  variables.value = (data.variables as string[]) || []
  const templates = (data.templates as Record<string, Record<string, LocalizedTemplate>>) || {}
  form.templates = {}
  scenes.value.forEach((scene) => {
    const sceneForm: Record<string, LocalizedTemplate> = {}
    locales.value.forEach((locale) => {
      const src = templates[scene]?.[locale]
      sceneForm[locale] = { subject: src?.subject || '', body: src?.body || '' }
    })
    form.templates[scene] = sceneForm
  })
  if (!currentScene.value || !scenes.value.includes(currentScene.value)) {
    currentScene.value = scenes.value[0] || ''
  }
}

const load = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getVerifyEmailTemplateSettings()
    applyResponse((res.data?.data || {}) as Record<string, unknown>)
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.alerts.fetchFailed'))
  } finally {
    loading.value = false
  }
}

const save = async () => {
  submitting.value = true
  try {
    const templates: Record<string, Record<string, LocalizedTemplate>> = {}
    scenes.value.forEach((scene) => {
      const sceneOut: Record<string, LocalizedTemplate> = {}
      locales.value.forEach((locale) => {
        const c = cell(scene, locale)
        sceneOut[locale] = { subject: c.subject, body: c.body }
      })
      templates[scene] = sceneOut
    })
    const res = await adminAPI.updateVerifyEmailTemplateSettings({ templates })
    applyResponse((res.data?.data || {}) as Record<string, unknown>)
    notifySuccess(t('admin.settings.alerts.saveSuccess'))
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.verifyEmailTemplate.saveFailed'))
  } finally {
    submitting.value = false
  }
}

const resetToDefault = async () => {
  if (!confirm(t('admin.settings.verifyEmailTemplate.resetConfirm'))) return
  resetting.value = true
  try {
    const res = await adminAPI.resetVerifyEmailTemplateSettings()
    applyResponse((res.data?.data || {}) as Record<string, unknown>)
    notifySuccess(t('admin.settings.verifyEmailTemplate.resetSuccess'))
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.alerts.saveFailed'))
  } finally {
    resetting.value = false
  }
}

onMounted(load)
defineExpose({ save, submitting })
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">{{ t('admin.settings.verifyEmailTemplate.title') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.verifyEmailTemplate.subtitle') }}</p>
      </div>

      <div class="space-y-6 p-6">
        <!-- 可用变量 -->
        <div class="rounded-lg border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-950/30">
          <h3 class="mb-2 text-sm font-medium text-blue-800 dark:text-blue-300">
            {{ t('admin.settings.verifyEmailTemplate.variables') }}
          </h3>
          <div class="flex flex-wrap gap-2">
            <span
              v-for="v in variables"
              :key="v"
              class="inline-flex items-center gap-1 rounded-md bg-blue-100 px-2 py-1 text-xs font-mono dark:bg-blue-900/50"
            >
              <code class="text-blue-700 dark:text-blue-300" v-text="'{{' + v + '}}'"></code>
              <span class="text-blue-600 dark:text-blue-400">{{ t(`admin.settings.verifyEmailTemplate.vars.${v}`) }}</span>
            </span>
          </div>
          <p class="mt-2 text-xs text-blue-600 dark:text-blue-400">
            {{ t('admin.settings.verifyEmailTemplate.brandHint') }}
          </p>
        </div>

        <!-- 场景选择 -->
        <div class="space-y-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.verifyEmailTemplate.scene') }}</label>
          <Select v-model="currentScene">
            <SelectTrigger class="w-full sm:w-72">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="s in scenes" :key="s" :value="s">
                {{ t(`admin.settings.verifyEmailTemplate.scenes.${s}`) }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <!-- 当前场景 + 当前语言 -->
        <div v-if="currentScene" class="space-y-4 rounded-lg border border-border p-4">
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.verifyEmailTemplate.subject') }}</label>
            <Input v-model="cell(currentScene, props.currentLang).subject" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.verifyEmailTemplate.body') }}</label>
            <Textarea v-model="cell(currentScene, props.currentLang).body" rows="8" class="font-mono text-sm" />
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.verifyEmailTemplate.codeRequired') }}</p>
          </div>
        </div>

        <div class="flex items-center gap-3">
          <Button :disabled="loading || submitting || resetting" @click="save">
            <span v-if="submitting" class="mr-2 h-3 w-3 animate-spin rounded-full border-2 border-primary/30 border-t-primary"></span>
            {{ submitting ? t('admin.settings.actions.saving') : t('admin.settings.actions.save') }}
          </Button>
          <Button variant="outline" :disabled="loading || submitting || resetting" @click="resetToDefault">
            <span v-if="resetting" class="mr-2 h-3 w-3 animate-spin rounded-full border-2 border-primary/30 border-t-primary"></span>
            {{ t('admin.settings.verifyEmailTemplate.resetToDefault') }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
