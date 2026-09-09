<template>
  <div id="app" class="min-h-screen bg-background text-foreground flex flex-col">
    <!-- vault 模板：自带顶栏/页脚的外壳包裹页面（控制台仍走下方分支） -->
    <VaultLayout v-if="isVault && !isResellerConsole">
      <ErrorBoundary>
        <RouterView v-slot="{ Component }">
          <Transition name="page-fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </ErrorBoundary>
    </VaultLayout>

    <!-- classic 模板 / 分销控制台（保持原有结构不变） -->
    <template v-else>
      <Navbar v-if="!isResellerConsole" />
      <main class="flex-1" :class="isResellerConsole ? '' : 'pb-14 lg:pb-0'">
        <ErrorBoundary>
          <RouterView v-slot="{ Component }">
            <Transition name="page-fade" mode="out-in">
              <component :is="Component" />
            </Transition>
          </RouterView>
        </ErrorBoundary>
      </main>
      <Footer v-if="!isResellerConsole" />
      <BackToTop v-if="!isResellerConsole" />
      <MobileBottomNav v-if="!isResellerConsole && !hideMobileBottomNav" />
    </template>

    <Loading :loading="appStore.loading" />
    <Toast />
    <ConfirmDialog />
    <!-- 客服浮窗：登录后全站常驻。分销控制台有自己的工作台，不挂。
         classic 模板底部有 MobileBottomNav，移动端需要抬高避免遮挡。 -->
    <SupportChatWidget
      v-if="showSupportWidget"
      :offset-bottom-mobile="!isVault"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from './stores/app'
import { useUserAuthStore } from './stores/userAuth'
import { useUserProfileStore } from './stores/userProfile'
import { getActiveTemplate } from './templates/registry'
import Navbar from './components/Navbar.vue'
import Footer from './components/Footer.vue'
import Loading from './components/Loading.vue'
import Toast from './components/Toast.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import ErrorBoundary from './components/ErrorBoundary.vue'
import BackToTop from './components/BackToTop.vue'
import MobileBottomNav from './components/MobileBottomNav.vue'

// vault 外壳按需加载，classic 用户不会拉取其 chunk/样式
const VaultLayout = defineAsyncComponent(() => import('./templates/vault/layout/VaultLayout.vue'))
// 客服浮窗按需加载：未登录访客不拉取该 chunk
const SupportChatWidget = defineAsyncComponent(() => import('./components/support/SupportChatWidget.vue'))

// config 由 router.beforeEach 统一加载，无需在此重复调用
const appStore = useAppStore()
const route = useRoute()
const isResellerConsole = computed(() => route.meta.resellerConsole === true)
// 商品详情页底部常驻购买条（加入购物车/立即购买/积分兑换），与主导航同为
// fixed bottom-0，两条并存会互相遮挡。这里让详情页只保留购买条——
// 和淘宝等电商详情页的做法一致，返回入口由页内的返回按钮承担。
const hideMobileBottomNav = computed(() => route.name === 'product-detail')
// getActiveTemplate 读取 appStore.config（响应式），config 加载后会重新计算
const isVault = computed(() => getActiveTemplate() === 'vault')
const userAuthStore = useUserAuthStore()
const userProfileStore = useUserProfileStore()
const showSupportWidget = computed(() => userAuthStore.isAuthenticated && !isResellerConsole.value)

const loadAuthedProfile = () => {
  if (userAuthStore.isAuthenticated) {
    void userProfileStore.loadProfile()
  }
}
onMounted(loadAuthedProfile)
watch(() => userAuthStore.isAuthenticated, loadAuthedProfile)
</script>

<style>
.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 200ms ease;
}

.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}
</style>
