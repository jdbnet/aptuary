<script setup>
defineProps({
  size: {
    type: String,
    default: 'md',
  },
})

const sizes = {
  sm: 'h-9 w-9',
  md: 'h-9 w-9',
  lg: 'h-14 w-14',
}
</script>

<template>
  <img
    src="/favicon.png"
    alt="Aptuary"
    class="shrink-0 rounded-lg object-contain"
    :class="sizes[size] || sizes.md"
    width="56"
    height="56"
  />
</template>
