import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { postAPI } from '../api'
import { debounceAsync } from '../utils/debounce'
import { useLocalized } from './useProduct'
import { usePageSeo } from './usePageSeo'

/**
 * 文章/公告详情共享逻辑（classic + vault 双模板共用）。
 * 文章详情（公告 / 常见问题）共用的详情逻辑。
 */
export function usePostDetail() {
  const route = useRoute()
  const { t } = useI18n()
  const appStore = useAppStore()
  const { formatPrice } = useLocalized()

  const loading = ref(true)
  const post = ref<any>(null)
  const relatedProducts = computed<any[]>(() => post.value?.related_products || [])

  const getLocalizedText = (jsonData: any) => {
    if (!jsonData) return ''
    const locale = appStore.locale
    return jsonData[locale] || jsonData['zh-CN'] || jsonData['en-US'] || ''
  }

  const formatDate = (dateString: string) => {
    if (!dateString) return ''
    const date = new Date(dateString)
    return date.toLocaleDateString(appStore.locale, {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    })
  }

  usePageSeo({
    title: () => post.value ? getLocalizedText(post.value.title) : '',
    description: () => post.value ? getLocalizedText(post.value.summary) : '',
    image: () => post.value?.thumbnail || '',
    canonicalPath: () => `/posts/${(route.params.slug as string) || ''}`,
    type: () => 'article',
  })

  // 详情页只承载公告与常见问题两类：公告回公告列表，其余（含常见问题）回首页。
  const backLink = computed(() => (post.value?.type === 'notice' ? '/notice' : '/'))

  const backText = computed(() =>
    post.value?.type === 'notice' ? t('postDetail.backToNotice') : t('nav.home'),
  )

  const loadPost = async () => {
    loading.value = true
    try {
      const slug = route.params.slug as string
      const response = await postAPI.detail(slug)
      post.value = response.data.data || null
    } catch (error) {
      console.error('Failed to load post:', error)
      post.value = null
    } finally {
      loading.value = false
    }
  }

  const debouncedLoadPost = debounceAsync(loadPost, 300)

  onMounted(() => {
    loadPost()
  })

  onUnmounted(() => {
    debouncedLoadPost.cancel()
  })

  return {
    loading,
    post,
    relatedProducts,
    getLocalizedText,
    formatDate,
    formatPrice,
    backLink,
    backText,
  }
}
