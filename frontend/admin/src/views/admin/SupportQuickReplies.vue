<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminSupportQuickReply } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import { Textarea } from '@/components/ui/textarea'
import { useAdminAuthStore } from '@/stores/auth'
import { confirmAction } from '@/utils/confirm'
import { formatDate } from '@/utils/format'
import { notifySuccess } from '@/utils/notify'

const { t } = useI18n()
const authStore = useAdminAuthStore()

const loading = ref(true)
const submitting = ref(false)
const showModal = ref(false)
const isEditing = ref(false)
const formError = ref('')
const replies = ref<AdminSupportQuickReply[]>([])
const editingId = ref<number | null>(null)
const form = reactive({
  title: '',
  content: '',
  is_active: true,
  sort_order: 0,
})

const canCreate = computed(() => authStore.hasPermission('POST:/admin/support/quick-replies'))
const canUpdate = computed(() => authStore.hasPermission('PUT:/admin/support/quick-replies/:id'))
const canDelete = computed(() => authStore.hasPermission('DELETE:/admin/support/quick-replies/:id'))
const canWrite = computed(() => canCreate.value || canUpdate.value)

const asInt = (value: unknown) => {
  const n = Math.trunc(Number(value))
  return Number.isFinite(n) ? n : 0
}

const resetForm = () => {
  form.title = ''
  form.content = ''
  form.is_active = true
  form.sort_order = 0
  formError.value = ''
  editingId.value = null
}

const fetchReplies = async () => {
  loading.value = true
  try {
    const response = await adminAPI.getSupportQuickReplies()
    const list = (response.data.data || []) as AdminSupportQuickReply[]
    replies.value = [...list].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
  } catch {
    replies.value = []
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  isEditing.value = false
  resetForm()
  showModal.value = true
}

const openEdit = (row: AdminSupportQuickReply) => {
  isEditing.value = true
  editingId.value = row.id
  form.title = row.title
  form.content = row.content
  form.is_active = row.is_active
  form.sort_order = row.sort_order
  formError.value = ''
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  resetForm()
}

const payload = () => ({
  title: form.title.trim(),
  content: form.content.trim(),
  is_active: form.is_active,
  sort_order: asInt(form.sort_order),
})

const submit = async () => {
  const data = payload()
  if (!data.title) {
    formError.value = t('admin.supportQuickReplies.errors.titleRequired')
    return
  }
  if (data.title.length > 120) {
    formError.value = t('admin.supportQuickReplies.errors.titleTooLong')
    return
  }
  if (data.content.length > 4000) {
    formError.value = t('admin.supportQuickReplies.errors.contentTooLong')
    return
  }
  if (isEditing.value && (!canUpdate.value || !editingId.value)) return
  if (!isEditing.value && !canCreate.value) return

  submitting.value = true
  formError.value = ''
  try {
    if (isEditing.value && editingId.value) {
      await adminAPI.updateSupportQuickReply(editingId.value, data)
    } else {
      await adminAPI.createSupportQuickReply(data)
    }
    notifySuccess(t('admin.supportQuickReplies.saveSuccess'))
    closeModal()
    await fetchReplies()
  } catch {
    // client 已提示
  } finally {
    submitting.value = false
  }
}

const remove = async (row: AdminSupportQuickReply) => {
  if (!canDelete.value) return
  const confirmed = await confirmAction({
    description: t('admin.supportQuickReplies.deleteConfirm', { title: row.title }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteSupportQuickReply(row.id)
    notifySuccess(t('admin.supportQuickReplies.deleteSuccess'))
    await fetchReplies()
  } catch {
    // client 已提示
  }
}

onMounted(fetchReplies)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold">{{ t('admin.supportQuickReplies.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.supportQuickReplies.subtitle') }}</p>
      </div>
      <Button v-if="canCreate" @click="openCreate">{{ t('admin.supportQuickReplies.create') }}</Button>
    </div>

    <div class="rounded-xl border border-border bg-card shadow-sm">
      <TableSkeleton v-if="loading" :rows="4" />
      <p v-else-if="replies.length === 0" class="p-6 text-sm text-muted-foreground">
        {{ t('admin.supportQuickReplies.empty') }}
      </p>
      <Table v-else>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('admin.supportQuickReplies.table.title') }}</TableHead>
            <TableHead>{{ t('admin.supportQuickReplies.table.content') }}</TableHead>
            <TableHead>{{ t('admin.supportQuickReplies.table.status') }}</TableHead>
            <TableHead>{{ t('admin.supportQuickReplies.table.sortOrder') }}</TableHead>
            <TableHead>{{ t('admin.supportQuickReplies.table.updatedAt') }}</TableHead>
            <TableHead v-if="canWrite || canDelete">{{ t('admin.supportQuickReplies.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="row in replies" :key="row.id">
            <TableCell class="font-medium">{{ row.title }}</TableCell>
            <TableCell class="max-w-sm truncate text-muted-foreground">{{ row.content }}</TableCell>
            <TableCell>
              <span :class="row.is_active ? 'theme-badge-success' : 'theme-badge-warning'">
                {{ row.is_active ? t('admin.supportQuickReplies.statusActive') : t('admin.supportQuickReplies.statusInactive') }}
              </span>
            </TableCell>
            <TableCell>{{ row.sort_order }}</TableCell>
            <TableCell class="text-muted-foreground">{{ formatDate(row.updated_at) }}</TableCell>
            <TableCell v-if="canWrite || canDelete">
              <div class="flex gap-2">
                <Button v-if="canUpdate" variant="outline" size="sm" @click="openEdit(row)">
                  {{ t('admin.common.edit') }}
                </Button>
                <Button v-if="canDelete" variant="destructive" size="sm" @click="remove(row)">
                  {{ t('admin.common.delete') }}
                </Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <Dialog v-model:open="showModal" @update:open="(value: boolean) => { if (!value) closeModal() }">
      <DialogScrollContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {{ isEditing ? t('admin.supportQuickReplies.edit') : t('admin.supportQuickReplies.create') }}
          </DialogTitle>
        </DialogHeader>
        <div class="space-y-4">
          <p v-if="formError" class="text-xs text-destructive">{{ formError }}</p>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.supportQuickReplies.form.title') }}
            </label>
            <Input v-model="form.title" maxlength="120" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.supportQuickReplies.form.titleHint') }}</p>
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.supportQuickReplies.form.content') }}
            </label>
            <Textarea v-model="form.content" class="min-h-[120px]" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.supportQuickReplies.form.contentHint') }}</p>
          </div>
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-sm font-medium">{{ t('admin.supportQuickReplies.form.isActive') }}</p>
            </div>
            <Switch v-model="form.is_active" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.supportQuickReplies.form.sortOrder') }}
            </label>
            <Input
              type="number"
              step="1"
              class="sm:max-w-xs"
              :model-value="form.sort_order"
              @update:model-value="form.sort_order = asInt($event)"
            />
          </div>
          <div class="flex justify-end gap-2">
            <Button variant="outline" @click="closeModal">{{ t('admin.common.cancel') }}</Button>
            <Button :disabled="submitting" @click="submit">{{ t('admin.common.save') }}</Button>
          </div>
        </div>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
