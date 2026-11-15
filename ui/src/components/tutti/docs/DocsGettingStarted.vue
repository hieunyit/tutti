<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  /**
   * Short getting-started: what Tutti is, the three first steps and where the
   * long deployment and operations guides live (the repository, not the app).
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nDocs from './i18nDocs';
  import { DOCS, GITHUB_BLOB_URL } from '@/utils/helpers/DocsHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nDocs });

  const steps = computed(() => [
    {
      key: 'step1',
      title: t('step1Title'),
      text: t('step1Text'),
      link: t('step1Link'),
      to: { name: RouteName.TUTTI_STORAGES },
    },
    {
      key: 'step2',
      title: t('step2Title'),
      text: t('step2Text'),
      link: t('step2Link'),
      to: { name: RouteName.TUTTI_ADD_REPLICATION },
    },
    {
      key: 'step3',
      title: t('step3Title'),
      text: t('step3Text'),
      link: t('step3Link'),
      to: { name: RouteName.TUTTI_REPLICATION },
    },
  ]);
  const guides = computed(() =>
    DOCS.map((doc) => ({
      slug: doc.slug,
      label: t(`guide_${doc.slug}`),
      href: `${GITHUB_BLOB_URL}${doc.path}`,
    })),
  );
</script>

<template>
  <div class="docs-start">
    <p class="docs-start__intro">{{ t('intro') }}</p>

    <section aria-labelledby="docs-start-steps">
      <h2 id="docs-start-steps">{{ t('stepsTitle') }}</h2>
      <ol class="docs-start__steps">
        <li
          v-for="step in steps"
          :key="step.key"
          :data-test="step.key"
        >
          <h3>{{ step.title }}</h3>
          <p>{{ step.text }}</p>
          <RouterLink :to="step.to">{{ step.link }}</RouterLink>
        </li>
      </ol>
    </section>

    <section aria-labelledby="docs-start-help">
      <h2 id="docs-start-help">{{ t('helpTitle') }}</h2>
      <p>{{ t('helpText') }}</p>
      <ul class="docs-start__guides">
        <li
          v-for="guide in guides"
          :key="guide.slug"
        >
          <a
            :href="guide.href"
            target="_blank"
            rel="noopener noreferrer"
            >{{ guide.label }}</a
          >
        </li>
      </ul>
      <RouterLink :to="{ name: RouteName.TUTTI_API_REFERENCE }">{{
        t('apiLink')
      }}</RouterLink>
    </section>
  </div>
</template>

<style lang="scss" scoped>
  .docs-start {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-6);
    max-width: var(--tt-measure);

    h2 {
      margin: 0 0 var(--tt-space-3);
      font-size: var(--tt-text-lg);
      font-weight: var(--tt-weight-strong);
    }

    h3 {
      margin: 0;
      font-size: var(--tt-text-md);
      font-weight: var(--tt-weight-strong);
    }

    p {
      margin: 0;
      color: var(--tt-text-2);
    }

    a {
      color: var(--tt-accent);
      text-decoration: underline;
    }

    &__intro {
      font-size: var(--tt-text-md);
    }

    &__steps {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-4);
      margin: 0;
      padding-left: var(--tt-space-5);

      li {
        display: flex;
        flex-direction: column;
        gap: var(--tt-space-1);
        padding-left: var(--tt-space-1);
      }
    }

    &__guides {
      margin: var(--tt-space-3) 0;
      padding-left: var(--tt-space-5);
    }
  }
</style>
