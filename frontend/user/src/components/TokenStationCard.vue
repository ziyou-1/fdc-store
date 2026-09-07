<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { KeyRound } from 'lucide-vue-next'
import { tokenStationAPI } from '../api'
import { Button } from '@/components/ui/button'

type PublicStation = {
  name: string
  url: string
}

const { t } = useI18n()
const stations = ref<PublicStation[]>([])
const visible = computed(() => stations.value.length > 0)

const load = async () => {
  const res = await tokenStationAPI.catalog()
  stations.value = res.data.data?.stations ?? []
}

onMounted(load)
</script>

<template>
  <section v-if="visible" class="space-y-3">
    <p class="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.18em] text-primary">
      <KeyRound class="h-3.5 w-3.5" aria-hidden="true" />
      {{ t('tokenStation.eyebrow') }}
    </p>
    <div class="grid gap-3 sm:grid-cols-2">
      <article
        v-for="station in stations"
        :key="station.url"
        class="rounded-2xl border border-primary/20 bg-primary/5 p-5"
      >
        <h2 class="truncate text-lg font-black text-foreground">{{ station.name }}</h2>
        <p class="mt-1 break-all font-mono text-[11px] text-muted-foreground">{{ station.url }}</p>
        <Button as-child class="mt-4 w-full sm:w-auto">
          <a :href="station.url" target="_blank" rel="noopener noreferrer">
            {{ t('tokenStation.open') }}
          </a>
        </Button>
      </article>
    </div>
  </section>
</template>
