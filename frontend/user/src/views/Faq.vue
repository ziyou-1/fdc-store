<template>
  <div class="faq-page min-h-screen bg-background pb-16 pt-20 text-foreground">
    <div class="container mx-auto px-4">
      <div class="mb-16 mt-12 text-center">
        <h1 class="mb-6 text-4xl font-black tracking-tight text-foreground md:text-6xl">{{ t('nav.faq') }}</h1>
        <p class="mx-auto max-w-2xl border-b pb-8 text-lg leading-relaxed text-muted-foreground">
          {{ t('faq.subtitle') }}
        </p>
      </div>

      <div v-if="loading" class="mx-auto max-w-4xl space-y-4">
        <div v-for="i in 6" :key="i" class="h-20 animate-pulse rounded-2xl border bg-muted"></div>
      </div>

      <!-- 答案很短，内联展开即可；不做详情页，也就不需要第二套详情壳与 sitemap 条目 -->
      <div v-else-if="posts.length > 0" class="mx-auto max-w-4xl space-y-3">
        <article
          v-for="item in posts"
          :key="item.id"
          class="overflow-hidden rounded-2xl border bg-card shadow-sm transition-shadow hover:shadow-md"
        >
          <h2>
            <button
              type="button"
              class="flex w-full items-center gap-4 px-6 py-5 text-left"
              :aria-expanded="isOpen(item.id)"
              :aria-controls="`faq-answer-${item.id}`"
              @click="toggle(item.id)"
            >
              <span
                class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border bg-secondary text-primary"
              >
                <HelpCircle class="h-5 w-5" />
              </span>
              <span class="min-w-0 flex-1 text-base font-bold text-foreground sm:text-lg">
                {{ getLocalizedText(item.title) }}
              </span>
              <ChevronDown
                class="h-5 w-5 shrink-0 text-muted-foreground transition-transform duration-200"
                :class="{ 'rotate-180': isOpen(item.id) }"
              />
            </button>
          </h2>

          <div v-show="isOpen(item.id)" :id="`faq-answer-${item.id}`" class="border-t px-6 py-5">
            <!-- 与博客/公告详情页同一条渲染链路：processHtmlForDisplay 负责把
                 富文本里的 /uploads/... 图片路径改写成可访问 URL，漏了图片就不显示 -->
            <div
              v-if="getLocalizedText(item.content)"
              class="prose prose-sm dark:prose-invert theme-prose max-w-none"
              v-html="processHtmlForDisplay(getLocalizedText(item.content))"
            ></div>
            <p v-else class="text-sm text-muted-foreground">
              {{ getLocalizedText(item.summary) || '-' }}
            </p>
          </div>
        </article>

        <PaginationNav :current-page="currentPage" :total-pages="totalPages" @change-page="changePage" />
      </div>

      <EmptyState v-else variant="soft" size="lg" class="mx-auto max-w-4xl" :title="t('faq.empty')">
        <template #icon>
          <HelpCircle class="h-20 w-20 text-muted-foreground opacity-70" :stroke-width="1.5" />
        </template>
      </EmptyState>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, HelpCircle } from 'lucide-vue-next'
import { processHtmlForDisplay } from '../utils/content'
import EmptyState from '../components/EmptyState.vue'
import PaginationNav from '../components/PaginationNav.vue'
import { usePostList } from '../composables/usePostList'

const { t } = useI18n()

const { loading, posts, currentPage, totalPages, getLocalizedText, changePage } = usePostList('faq', {
  title: () => t('nav.faq'),
  canonicalPath: '/faq',
})

// 允许同时展开多条：用户常常要对照两个答案
const openIds = ref<Set<number>>(new Set())
const isOpen = (id: number) => openIds.value.has(id)
const toggle = (id: number) => {
  const next = new Set(openIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  openIds.value = next
}
</script>
