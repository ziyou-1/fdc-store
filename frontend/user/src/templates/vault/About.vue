<template>
  <div class="mx-auto w-full max-w-[840px] px-6 pb-10 pt-2">
    <header class="my-8 text-center">
      <h1 class="mb-2.5 text-4xl font-extrabold">{{ heroTitle }}</h1>
      <p class="text-muted-foreground">{{ heroSubtitle }}</p>
    </header>

    <Card class="p-7 sm:p-[34px]">
      <div v-if="hasIntroduction" class="mb-9 last:mb-0">
        <p class="whitespace-pre-line text-base leading-[1.8] text-muted-foreground">{{ introductionText }}</p>
      </div>

      <div v-if="hasServices" class="mb-9 last:mb-0">
        <h2 v-if="servicesTitle" class="mb-[18px] flex items-center gap-3 text-[22px] font-bold">
          <span class="h-[26px] w-[5px] flex-none rounded-full bg-primary"></span>{{ servicesTitle }}
        </h2>
        <div class="grid gap-3.5 sm:grid-cols-2">
          <div v-for="(service, index) in serviceItems" :key="`about-service-${index}`" class="flex items-start gap-3 rounded-md border bg-secondary px-4 py-3.5">
            <Check class="mt-0.5 h-5 w-5 flex-none text-[color:var(--teal-strong)]" />
            <span class="text-[14.5px] text-muted-foreground">{{ service }}</span>
          </div>
        </div>
      </div>

      <div v-if="hasContact" class="mb-9 last:mb-0">
        <h2 v-if="contactTitle" class="mb-[18px] flex items-center gap-3 text-[22px] font-bold">
          <span class="h-[26px] w-[5px] flex-none rounded-full bg-primary"></span>{{ contactTitle }}
        </h2>
        <p v-if="contactText" class="mb-[18px] whitespace-pre-line leading-relaxed text-muted-foreground">{{ contactText }}</p>
      </div>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { Check } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import { useAbout } from '../../composables/useAbout'

const {
  heroTitle, heroSubtitle, introductionText, servicesTitle, contactTitle, contactText,
  serviceItems, hasIntroduction, hasServices, hasContact,
} = useAbout()
</script>
