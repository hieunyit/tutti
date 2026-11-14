import type { TuttiReplication } from '@/utils/types/tuttiCore';

type Num = string | number | undefined | null;

export interface InitProgress {
  /** false when the server reports no bytes (e.g. versioned buckets): show object counts only */
  tracked: boolean;
  doneBytes: number;
  totalBytes: number;
  /** 0..100 with one decimal, null when total is unknown */
  percent: number | null;
  bytesPerSecond: number;
  /** null when unknown (listing running, no throughput) */
  etaSeconds: number | null;
  /** false: totalBytes is only what was listed so far ("at least") */
  listingComplete: boolean;
}

export class InitProgressHelper {
  /** int64 fields arrive as strings; anything unusable is 0. */
  static toNumber(value: Num): number {
    const n = typeof value === 'number' ? value : Number(value);

    return Number.isFinite(n) && n > 0 ? n : 0;
  }

  /** 1024 based: "812 GB", "12.4 GB", "0 B". */
  static formatSize(bytes: number): string {
    const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    let v = Math.max(0, bytes);
    let i = 0;

    while (v >= 1024 && i < units.length - 1) {
      v /= 1024;
      i += 1;
    }

    return `${v >= 100 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`;
  }

  /** "25 h 10 min", "3 d 4 h", "5 min", "< 1 min"; null when unknown. */
  static formatDuration(seconds: number | null): string | null {
    if (seconds === null || !Number.isFinite(seconds) || seconds <= 0) {
      return null;
    }

    const minutes = Math.round(seconds / 60);

    if (minutes < 1) return '< 1 min';

    if (minutes < 60) return `${minutes} min`;

    const hours = Math.floor(minutes / 60);
    const restMin = minutes % 60;

    if (hours < 48) return restMin ? `${hours} h ${restMin} min` : `${hours} h`;

    const days = Math.floor(hours / 24);
    const restHours = hours % 24;

    return restHours ? `${days} d ${restHours} h` : `${days} d`;
  }

  static fromReplication(r: TuttiReplication): InitProgress {
    const totalBytes = InitProgressHelper.toNumber(r.initBytesTotal);
    const doneBytes = InitProgressHelper.toNumber(r.initBytesDone);
    const eta = InitProgressHelper.toNumber(r.initEtaSeconds);

    return {
      tracked: totalBytes > 0,
      doneBytes,
      totalBytes,
      percent:
        totalBytes > 0
          ? Math.min(100, Math.round((doneBytes / totalBytes) * 1000) / 10)
          : null,
      bytesPerSecond: InitProgressHelper.toNumber(r.initBytesPerSecond),
      etaSeconds: eta > 0 ? eta : null,
      listingComplete: r.initListingComplete === true,
    };
  }
}
