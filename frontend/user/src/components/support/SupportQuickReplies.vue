<script setup lang="ts">
import type { SupportQuickReplyData } from '../../api/types'

defineProps<{ replies: SupportQuickReplyData[] }>()
const emit = defineEmits<{ (e: 'pick', reply: SupportQuickReplyData): void }>()
</script>

<template>
  <div v-if="replies.length > 0" class="flex flex-wrap gap-1.5 border-t border-border px-4 py-2">
    <button
      v-for="reply in replies"
      :key="reply.id"
      type="button"
      class="rounded-full border border-border bg-background px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
      @click="emit('pick', reply)"
    >
      {{ reply.title }}
    </button>
  </div>
</template>
