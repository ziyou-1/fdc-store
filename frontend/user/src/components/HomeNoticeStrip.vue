<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Bell, ChevronRight } from 'lucide-vue-next'
import { postAPI } from '../api/product'
import { useNavConfig } from '../composables/useNavConfig'
import { useLocalized } from '../composables/useProduct'

const { t } = useI18n()
const router = useRouter()
const { noticeEnabled } = useNavConfig()
const { getLocalizedText } = useLocalized()

const notices = ref<any[]>([])

// 公告开关关掉、或一条公告都没有时整条不渲染。首页不留空占位。
const visible = computed(() => noticeEnabled.value && notices.value.length > 0)

const formatDate = (value?: string) => {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString()
}

// 公告与常见问题详情共用 /posts/:slug
const goToNotice = (slug: string) => router.push(`/posts/${encodeURIComponent(slug)}`)

onMounted(async () => {
  if (!noticeEnabled.value) return
  try {
    const response = await postAPI.list({ page: 1, page_size: 3, type: 'notice' })
    notices.value = response.data.data || []
  } catch {
    // 首页组件失败必须静默：一条公告条不该把首页搞出报错
    notices.value = []
  }
})
</script>

<template>
  <section v-if="visible" class="rounded-2xl border bg-card p-4 shadow-sm sm:p-5">
    <div class="mb-3 flex items-center justify-between gap-3">
      <h2 class="flex items-center gap-2 text-sm font-bold text-foreground">
        <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Bell class="h-4 w-4" aria-hidden="true" />
        </span>
        {{ t('nav.notice') }}
      </h2>
      <RouterLink to="/notice" class="text-xs text-muted-foreground transition-colors hover:text-foreground">
        {{ t('home.notice.viewAll') }}
      </RouterLink>
    </div>

    <ul class="divide-y">
      <li v-for="notice in notices" :key="notice.id">
        <button
          type="button"
          class="group flex w-full items-center gap-3 py-2.5 text-left"
          @click="goToNotice(notice.slug)"
        >
          <span class="min-w-0 flex-1 truncate text-sm text-foreground group-hover:text-primary">
            {{ getLocalizedText(notice.title) }}
          </span>
          <time class="shrink-0 font-mono text-[11px] text-muted-foreground">
            {{ formatDate(notice.published_at) }}
          </time>
          <ChevronRight
            class="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5"
            aria-hidden="true"
          />
        </button>
      </li>
    </ul>
  </section>
</template>
