<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /**
   * One titled block of a settings tab. Fields sit in a responsive grid
   * (2 columns from 1024px, 1 below); `list` stacks full-width rows instead
   * (switches). `collapsible` renders a closed-by-default disclosure that
   * opens by itself while `forceOpen` is set (e.g. a field inside has an error).
   */
  import { ref, watch } from 'vue';

  const props = defineProps<{
    title: string;
    description?: string;
    list?: boolean;
    collapsible?: boolean;
    forceOpen?: boolean;
  }>();

  const open = ref(false);

  watch(
    () => props.forceOpen,
    (value) => {
      if (value) open.value = true;
    },
    { immediate: true },
  );

  function onToggle(event: Event) {
    open.value = (event.target as HTMLDetailsElement).open;
  }
</script>

<template>
  <details
    v-if="collapsible"
    class="settings-section settings-section--collapsible"
    data-test="settings-section-collapsible"
    :open="open"
    @toggle="onToggle"
  >
    <summary class="settings-section__title">{{ title }}</summary>
    <p
      v-if="description"
      class="settings-section__desc"
    >
      {{ description }}
    </p>
    <div
      class="settings-section__body"
      :class="{ 'settings-section__body--list': list }"
    >
      <slot />
    </div>
  </details>
  <section
    v-else
    class="settings-section"
  >
    <h2 class="settings-section__title">{{ title }}</h2>
    <p
      v-if="description"
      class="settings-section__desc"
    >
      {{ description }}
    </p>
    <div
      class="settings-section__body"
      :class="{ 'settings-section__body--list': list }"
    >
      <slot />
    </div>
  </section>
</template>

<style lang="scss" scoped>
  .settings-section {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    min-width: 0;

    &__title {
      margin: 0;
      font-size: var(--tt-text-md, 1rem);
      font-weight: var(--tt-weight-semibold, 600);
      color: var(--tt-text);
    }

    &__desc {
      margin: calc(-1 * var(--tt-space-2)) 0 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
    }

    &__body {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: var(--tt-space-4) var(--tt-space-6);

      &--list {
        display: flex;
        flex-direction: column;
        gap: 0;
        padding: 0 var(--tt-space-4);
        border: 1px solid var(--tt-border-subtle);
        border-radius: var(--tt-radius-md, 8px);
      }

      @media (min-width: 1024px) {
        &:not(&--list) {
          grid-template-columns: repeat(2, minmax(0, 1fr));
        }
      }
    }

    &--collapsible {
      padding: var(--tt-space-3) var(--tt-space-4);
      border: 1px solid var(--tt-border-subtle);
      border-radius: var(--tt-radius-md, 8px);

      > .settings-section__title {
        cursor: pointer;
      }

      &[open] > .settings-section__title {
        margin-bottom: var(--tt-space-3);
      }

      > .settings-section__desc {
        margin: 0 0 var(--tt-space-3);
      }
    }
  }
</style>
