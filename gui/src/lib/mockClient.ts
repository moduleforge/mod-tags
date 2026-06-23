// In-memory mock of createTagsClient's return shape. Used only by Ladle stories
// so components render without a real backend. Not exported from the library.

import type { Tag } from './api';

interface MockClientOptions {
  initial?: Tag[];
  latencyMs?: number;
  failOn?: { list?: boolean; create?: boolean; update?: boolean; remove?: boolean };
}

function delay(ms: number) {
  return new Promise<void>((r) => setTimeout(r, ms));
}

let seq = 1;

export function createMockTagsClient(opts: MockClientOptions = {}) {
  const latency = opts.latencyMs ?? 120;
  const state = new Map<string, Tag>();
  for (const t of opts.initial ?? []) state.set(t.uuid, t);

  function nowIso() {
    return new Date().toISOString();
  }

  return {
    async listBySubject(subjectUuid: string, purposes?: string[]): Promise<Tag[]> {
      await delay(latency);
      if (opts.failOn?.list) throw new Error('Mock failure: listBySubject');
      const all = Array.from(state.values()).filter((t) => t.subjectUuid === subjectUuid);
      if (!purposes || purposes.length === 0) return all;
      const set = new Set(purposes);
      return all.filter((t) => set.has(t.purpose));
    },
    async create(input: {
      subject: string;
      purpose: string;
      value: string;
      color?: string;
    }): Promise<Tag> {
      await delay(latency);
      if (opts.failOn?.create) throw new Error('Mock failure: create');
      const uuid = `mock-${seq++}`;
      const now = nowIso();
      const tag: Tag = {
        uuid,
        ownerUuid: 'mock-owner',
        subjectUuid: input.subject,
        purpose: input.purpose,
        value: input.value,
        color: input.color,
        createdAt: now,
        updatedAt: now,
      };
      state.set(uuid, tag);
      return tag;
    },
    async updateColor(uuid: string, color: string | null): Promise<Tag> {
      await delay(latency);
      if (opts.failOn?.update) throw new Error('Mock failure: updateColor');
      const existing = state.get(uuid);
      if (!existing) throw new Error(`No tag ${uuid}`);
      const updated: Tag = {
        ...existing,
        color: color ?? undefined,
        updatedAt: nowIso(),
      };
      state.set(uuid, updated);
      return updated;
    },
    async updateValue(uuid: string, value: string): Promise<Tag> {
      await delay(latency);
      if (opts.failOn?.update) throw new Error('Mock failure: updateValue');
      const existing = state.get(uuid);
      if (!existing) throw new Error(`No tag ${uuid}`);
      const updated: Tag = {
        ...existing,
        value,
        updatedAt: nowIso(),
      };
      state.set(uuid, updated);
      return updated;
    },
    async remove(uuid: string): Promise<void> {
      await delay(latency);
      if (opts.failOn?.remove) throw new Error('Mock failure: remove');
      state.delete(uuid);
    },
  };
}
