<script setup lang="ts">
import { onMounted } from 'vue'

import { useAuthSession } from '@/composables/useAuthSession'

/**
 * 起動時に認証contextを取得する。
 * 同時にserverがCSRF token Cookieを発行する (設計書 18.4)。
 *
 * router guardも初期化を行うため、ここでの呼び出しは重複しても1回に集約される。
 */
const { initialize } = useAuthSession()

onMounted(() => {
  void initialize()
})
</script>

<template>
  <RouterView />
</template>
