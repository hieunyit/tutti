/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
export default {
  retry: 'Erneut versuchen',
  diffTitle: 'Änderungen vor dem Speichern prüfen',
  diffText: 'Diese Einstellungen werden als Version {version} gespeichert.',
  diffNoChanges: 'Keine Änderungen.',
  confirmSave: 'Einstellungen speichern',
  cancel: 'Abbrechen',
  conflictTitle: 'Einstellungen wurden von jemand anderem geändert',
  conflictText:
    'Ein anderer Operator hat eine neuere Version gespeichert. Neu laden, um dessen Werte zu sehen (Ihr Entwurf wird verworfen), oder überschreiben, um Ihren Entwurf auf der neueren Version zu speichern.',
  reload: 'Einstellungen neu laden',
  overwrite: 'Mit meinem Entwurf überschreiben',
  bannerFailed: 'Einstellungen konnten nicht gespeichert werden',
  tabFeatures: 'Replikationsfunktionen',
  tabBulk: 'Limits der Erstkopie',
  tabWorker: 'Worker',
  tabLocking: 'Sperren',
  tabWebhook: 'Webhook',
  tabProxies: 'Proxys',
  restartRequired: 'Worker-Neustart erforderlich',
  validationSummary: '{count} Feld(er) prüfen',
  reviewFields: 'Felder prüfen',
  requestFailed: 'Konfigurationsanfrage fehlgeschlagen',
  loadFailed:
    'Konfiguration konnte nicht geladen werden. Vor dem Bearbeiten erneut versuchen; es wurden keine Standardwerte angewendet.',
  draftRetained:
    'Ihr Entwurf bleibt erhalten. Vor dem erneuten Speichern die aktuelle Version laden.',
  discardTitle: 'Ungespeicherte Änderungen verwerfen?',
  discardText:
    'Der Entwurf wird ersetzt oder verlassen. Nur fortfahren, wenn diese Änderungen nicht mehr benötigt werden.',
  discardConfirm: 'Änderungen verwerfen',
  keepEditing: 'Weiter bearbeiten',
  durationHint: 'Zum Beispiel 500ms, 30s oder 1m30s.',
  invalidDuration: 'Positive Dauer eingeben, z. B. 500ms, 30s oder 1m30s.',
  invalidConcurrency:
    '0 (automatisch) oder eine ganze Zahl zwischen 1 und 1.000 eingeben.',
  invalidWatermark: 'Ganze Zahl zwischen 1 und 2.147.483.647 eingeben.',
  invalidWatermarkOrder: 'Die untere Grenze muss kleiner als die obere sein.',
  invalidBytes:
    'Nichtnegative ganze Bytezahl eingeben (maximal 9.223.372.036.854.775.807).',
  invalidUrl:
    'Absolute HTTP(S)-URL ohne Zugangsdaten eingeben oder leer lassen.',
  title: 'Systemeinstellungen',
  save: 'Einstellungen speichern',
  saving: 'Wird gespeichert...',
  reset: 'Änderungen verwerfen',
  tagging: 'Objekt- & Bucket-Tagging',
  taggingDesc: 'Tags an Buckets und Objekten replizieren.',
  acl: 'Zugriffskontrolllisten (ACL)',
  aclDesc: 'Berechtigungen und Zugriffsrechte replizieren.',
  lifecycle: 'Lebenszyklus-Konfiguration',
  lifecycleDesc: 'Ablauf- und Übergangsregeln replizieren.',
  policy: 'Bucket-Richtlinien',
  policyDesc: 'Bucket-Zugriffsrichtlinien replizieren.',
  preserveAclGrants: 'Quell-ACL-Berechtigungen beibehalten',
  preserveAclGrantsDesc:
    'Ursprüngliche Rechte behalten statt sie auf den Zielbesitzer abzubilden.',
  versioning: 'Objekt-Versionierung',
  versioningDesc: 'Versionen und Löschmarkierungen replizieren.',
  concurrency: 'Task-Parallelität',
  concurrencyHint: 'Parallele Replikationsaufgaben je Worker. 0 = automatisch.',
  shutdownTimeout: 'Timeout beim Herunterfahren',
  queueUpdateInterval: 'Warteschlangen-Aktualisierungsintervall',
  taskCheckInterval: 'Task-Prüfintervall',
  delayedTaskCheckInterval: 'Intervall verzögerter Tasks',
  swiftRetryInterval: 'Swift-Wiederholungsintervall',
  pauseRetryInterval: 'Pausen-Wiederholungsintervall',
  switchRetryInterval: 'Umschalt-Wiederholungsintervall',
  backlogHighWatermark: 'Backlog-Höchstgrenze (Tasks)',
  backlogHighWatermarkHint:
    'Auflistung pausieren, wenn der Kopierrückstand diesen Wert erreicht.',
  backlogLowWatermark: 'Backlog-Niedriggrenze (Tasks)',
  backlogLowWatermarkHint:
    'Auflistung darunter fortsetzen. Muss unter der oberen Grenze liegen.',
  bandwidthLimitBytes: 'Massenkopie-Bandbreite (Bytes/s)',
  bandwidthLimitHint: 'Je Worker. 0 = unbegrenzt.',
  bufferBudgetBytes: 'Pufferbudget im Flug (Bytes)',
  bufferBudgetHint:
    'Je Worker. 0 = automatisch (25 % des Speichers); sonst mindestens 16 MiB.',
  tuneTitle: 'Replikationsdurchsatz',
  tuneDesc: 'Ein leeres Feld behält den darin angezeigten automatischen Wert.',
  appliesLive: 'Wirkt sofort',
  bulkConcurrencyPercent: 'Anteil Erstkopie (%)',
  bulkConcurrencyPercentHint:
    'Anteil der Task-Parallelität für die Erstkopie. 0 = automatisch; 5 bis 95.',
  multipartThreads: 'Multipart-Threads',
  multipartThreadsHint:
    'Parallele Teil-Uploads je Objekt. 0 = automatisch; 1 bis 16.',
  multipartPartSizeMib: 'Multipart-Teilgröße (MiB)',
  multipartPartSizeMibHint:
    'Größe jedes hochgeladenen Teils. 0 = automatisch (16); 5 bis 512.',
  largeObjectStreams: 'Streams für große Objekte',
  largeObjectStreamsHint:
    'Gleichzeitige Kopien von Objekten ab 128 MiB je Ziel. 0 = automatisch; 1 bis 64.',
  invalidLargeStreams:
    '0 (automatisch) oder eine ganze Zahl von 1 bis 64 eingeben.',
  invalidBulkPercent:
    '0 (automatisch) oder eine ganze Zahl von 5 bis 95 eingeben.',
  invalidMultipartThreads:
    '0 (automatisch) oder eine ganze Zahl von 1 bis 16 eingeben.',
  invalidPartSize:
    '0 (automatisch) oder eine ganze Zahl von 5 bis 512 eingeben.',
  recTitle: 'Empfohlene Werte',
  recIntro:
    'Mit Auto markierte Werte stammen aus der erkannten Hardware. Übernehmen setzt einen Wert im Formular; Automatik nutzen setzt alles auf automatisch. Nichts ändert sich vor dem Speichern.',
  recDetected: 'Vom Worker erkannt: {cpu} CPU, {mem} Speicher ({source}).',
  recDetectedFractional: 'CPU-Kontingent {cpu}',
  recUnavailable: 'Der Worker hat seine Hardware nicht gemeldet.',
  recOverride: 'Vorschau für andere Hardware',
  recCores: 'CPU-Kerne je Worker',
  recRam: 'RAM je Worker (GB)',
  recFill: 'Übernehmen',
  recUseAuto: 'Automatik nutzen',
  autoBadge: 'Auto',
  autoBadgeTitle:
    'Aus der erkannten Hardware abgeleitet; Wert setzen zum Überschreiben.',
  recConcurrency: 'Task-Parallelität: etwa 10 je CPU, höchstens 128.',
  recBuffer: 'Pufferbudget: 25 % des Speichers, 256 MiB bis 8 GiB.',
  recThreads:
    'Multipart-Threads: 1 (Standard). Nur nach Messung im eigenen Netzwerk erhöhen; parallele Teile waren in lokalen Tests langsamer.',
  recBulk: 'Anteil Erstkopie: 20 % bis 3 CPUs, 30 % ab 4, 40 % ab 8.',
  throughputLink: 'Durchsatz-Panel öffnen',
  lockOverlap: 'Sperrüberlappungsdauer',
  lockOverlapHint: 'Sicherheitsabstand für Worker-Sperren, zum Beispiel 2s.',
  webhookEnabled: 'Webhook-Endpunkt aktivieren',
  webhookBaseUrl: 'Webhook-Basis-URL',
  webhookBaseUrlHint:
    'Öffentliche HTTP(S)-Adresse, die Speicherbenachrichtigungen empfängt.',
  conflictError:
    'Die Konfiguration wurde von einem anderen Benutzer geändert. Bitte neu laden.',
  featuresLead: 'Wählen Sie, was die Replikation außer den Objekten kopiert.',
  bulkLead: 'Limits je Worker für die Erstkopie und ihre Warteschlange.',
  statusSaved: 'Version {version} · gespeichert {when}',
  statusNotSaved: 'Noch nicht gespeichert',
  unsaved:
    '{count} ungespeicherte Änderung | {count} ungespeicherte Änderungen',
  webhookLead:
    'Ereignisbenachrichtigungen von S3- und Swift-Speichern empfangen.',
} as Record<string, string>;
