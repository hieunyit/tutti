<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /**
   * The one frame of every settings tab: a left-aligned column of at most
   * 960px with sections spaced evenly. An optional aside sits beside it from
   * 1280px (sticky) and below the sections on narrower screens.
   */
</script>

<template>
  <div
    class="settings-layout"
    :class="{ 'settings-layout--aside': $slots.aside }"
  >
    <div class="settings-layout__main">
      <slot />
    </div>
    <aside
      v-if="$slots.aside"
      class="settings-layout__aside"
    >
      <slot name="aside" />
    </aside>
  </div>
</template>

<style lang="scss" scoped>
  .settings-layout {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-6);
    max-width: 960px;
    min-width: 0;

    &__main {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-8, 32px);
      min-width: 0;
    }

    @media (min-width: 1280px) {
      &--aside {
        display: grid;
        grid-template-columns: minmax(0, 960px) 320px;
        align-items: start;
        max-width: calc(960px + 320px + var(--tt-space-6));
      }

      &__aside {
        position: sticky;
        top: var(--tt-space-4);
      }
    }
  }
</style>
