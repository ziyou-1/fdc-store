<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminTokenStation } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogScrollContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import { confirmAction } from '@/utils/confirm'
import { notifyError, notifySuccess } from '@/utils/notify'

const { t } = useI18n()
const loading = ref(true)
const stations = ref<AdminTokenStation[]>([])
const showModal = ref(false)
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const submitting = ref(false)
const form = reactive({
  name: '',
  base_url: '',
})

const resetForm = () => {
  form.name = ''
  form.base_url = ''
}

const fetchStations = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getTokenStations()
    stations.value = res.data.data || []
  } finally {
    loading.value = false
  }
}

const openCreateModal = () => {
  isEditing.value = false
  editingId.value = null
  resetForm()
  showModal.value = true
}

const openEditModal = (station: AdminTokenStation) => {
  isEditing.value = true
  editingId.value = station.id
  form.name = station.name
  form.base_url = station.base_url
  showModal.value = true
}

const handleSave = async () => {
  submitting.value = true
  try {
    const payload = {
      name: form.name,
      base_url: form.base_url,
    }
    if (isEditing.value && editingId.value) {
      await adminAPI.updateTokenStation(editingId.value, payload)
    } else {
      await adminAPI.createTokenStation(payload)
    }
    showModal.value = false
    notifySuccess()
    await fetchStations()
  } catch (err: any) {
    notifyError(err?.response?.data?.message || err?.message)
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (station: AdminTokenStation) => {
  const confirmed = await confirmAction({
    description: t('admin.tokenStations.deleteConfirm', { name: station.name }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteTokenStation(station.id)
    notifySuccess()
    await fetchStations()
  } catch (err: any) {
    notifyError(err?.response?.data?.message || err?.message)
  }
}

onMounted(fetchStations)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold">{{ t('admin.tokenStations.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.tokenStations.subtitle') }}</p>
      </div>
      <Button class="w-full sm:w-auto" @click="openCreateModal">{{ t('admin.tokenStations.create') }}</Button>
    </div>

    <div class="overflow-x-auto rounded-xl border border-border bg-card">
      <Table class="min-w-[720px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.tokenStations.table.id') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.tokenStations.table.name') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.tokenStations.table.baseUrl') }}</TableHead>
            <TableHead class="px-6 py-3 text-right">{{ t('admin.tokenStations.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell colspan="4" class="p-0">
              <TableSkeleton :columns="4" :rows="4" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="stations.length === 0">
            <TableCell colspan="4" class="px-6 py-8 text-center text-muted-foreground">
              {{ t('admin.tokenStations.empty') }}
            </TableCell>
          </TableRow>
          <TableRow v-for="station in stations" :key="station.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4"><IdCell :value="station.id" /></TableCell>
            <TableCell class="px-6 py-4 font-medium">{{ station.name }}</TableCell>
            <TableCell class="px-6 py-4">
              <a :href="station.base_url" class="break-all font-mono text-xs text-primary hover:underline" target="_blank" rel="noopener noreferrer">
                {{ station.base_url }}
              </a>
            </TableCell>
            <TableCell class="px-6 py-4 text-right">
              <div class="flex justify-end gap-2">
                <Button size="sm" variant="outline" @click="openEditModal(station)">{{ t('admin.common.edit') }}</Button>
                <Button size="sm" variant="destructive" @click="handleDelete(station)">{{ t('admin.common.delete') }}</Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <Dialog v-model:open="showModal">
      <DialogScrollContent>
        <DialogHeader>
          <DialogTitle>{{ isEditing ? t('admin.tokenStations.editTitle') : t('admin.tokenStations.createTitle') }}</DialogTitle>
        </DialogHeader>
        <div class="space-y-3">
          <div>
            <label class="mb-1 block text-xs text-muted-foreground">{{ t('admin.tokenStations.form.name') }}</label>
            <Input v-model="form.name" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-muted-foreground">{{ t('admin.tokenStations.form.baseUrl') }}</label>
            <Input v-model="form.base_url" placeholder="https://" />
          </div>
          <Button :disabled="submitting" @click="handleSave">
            {{ submitting ? t('admin.common.submitting') : t('admin.common.save') }}
          </Button>
        </div>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
