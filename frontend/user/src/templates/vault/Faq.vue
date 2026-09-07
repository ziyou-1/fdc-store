<template>
  <div class="mx-auto w-full max-w-[1180px] px-6 pb-10 pt-2">
    <header class="my-6 flex flex-col items-start gap-2">
      <h1 class="text-[34px] font-extrabold">{{ t('nav.faq') }}</h1>
      <p class="text-muted-foreground">{{ t('faq.subtitle') }}</p>
    </header>

    <!-- Loading -->
    <div v-if="loading" class="mx-auto grid max-w-[880px] gap-3.5">
      <div v-for="i in 6" :key="i" class="h-[76px] rounded-lg border bg-card opacity-50"></div>
    </div>

    <!-- FAQ 手风琴：答案很短，内联展开，不做详情页 -->
    <template v-else-if="posts.length > 0">
      <div class="mx-auto grid max-w-[880px] gap-3">
        <article
          v-for="item in posts"
          :key="item.id"
          class="overflow-hidden rounded-lg border bg-card transition hover:border-hairline-strong"
        >
          <h2>
            <button
              type="button"
              class="flex w-full items-center gap-[18px] px-[22px] py-[18px] text-left"
              :aria-expanded="isOpen(item.id)"
              :aria-controls="`vault-faq-answer-${item.id}`"
              @click="toggle(item.id)"
            >
              <span class="grid h-10 w-10 flex-none place-items-center rounded-md bg-secondary text-primary">
                <HelpCircle class="h-5 w-5" />
              </span>
              <span class="min-w-0 flex-1 text-base font-bold sm:text-lg">
                {{ getLocalizedText(item.title) }}
              </span>
              <ChevronDown
                class="h-5 w-5 flex-none text-muted-foreground transition-transform duration-200"
                :class="{ 'rotate-180': isOpen(item.id) }"
              />
            </button>
          </h2>

          <div v-show="isOpen(item.id)" :id="`vault-faq-answer-${item.id}`" class="border-t px-[22px] py-[18px]">
            <div
              v-if="getLocalizedText(item.content)"
              class="prose prose-sm dark:prose-invert theme-prose max-w-none"
              v-html="processHtmlForDisplay(getLocalizedText(item.content))"
            ></div>
            <p v-else class="text-sm text-muted-foreground">{{ getLocalizedText(item.summary) || '-' }}</p>
          </div>
        </article>
      </div>

      <div v-if="totalPages > 1" class="mt-8 flex items-center justify-center gap-3.5">
        <Button
          variant="outline"
          size="icon"
          class="rounded-full"
          :disabled="currentPage <= 1"
          @click="changePage(currentPage - 1)"
        >
          <ChevronLeft />
        </Button>
        <span class="text-sm font-bold">{{ currentPage }} / {{ totalPages }}</span>
        <Button
          variant="outline"
          size="icon"
          class="rounded-full"
          :disabled="currentPage >= totalPages"
          @click="changePage(currentPage + 1)"
        >
          <ChevronRight />
        </Button>
      </div>
    </template>

    <!-- Empty -->
    <div
      v-else
      class="flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground"
    >
      <HelpCircle class="h-10 w-10 opacity-60" />
      <p>{{ t('faq.empty') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, ChevronLeft, ChevronRight, HelpCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { processHtmlForDisplay } from '../../utils/content'
import { usePostList } from '../../composables/usePostList'

const { t } = useI18n()

const { loading, posts, currentPage, totalPages, getLocalizedText, changePage } = usePostList('faq', {
  title: () => t('nav.faq'),
  canonicalPath: '/faq',
})

const openIds = ref<Set<number>>(new Set())
const isOpen = (id: number) => openIds.value.has(id)
const toggle = (id: number) => {
  const next = new Set(openIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  openIds.value = next
}
</script>
