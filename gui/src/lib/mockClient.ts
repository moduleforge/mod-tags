// In-memory mock of createTagsClient's return shape. Used only by Ladle stories
// so components render without a real backend. Not exported from the library.
//
// Failures are thrown as `ApiRequestError` — matching what the real client
// throws once a request routes through `@moduleforge/core-gui`'s `request()`
// (task 001) — so stories exercise the same field/banner/toast surfaces
// `useApiError` routes production errors to (see the "Surface classification"
// table in docs/mf-standards/architecture/api-response-design.md).

import type { FieldErrorData, Tag } from './api';
import { ApiRequestError } from './api';

/**
 * The failure classes stories exercise, each chosen to demonstrate a
 * different row of the surface-classification table:
 * - `forbidden` — banner, no field details.
 * - `conflict` — banner, top-level "tag already exists", no field details.
 * - `invalid_input` — field-level, carries a `details` entry bound to `value`.
 * - `network_error` — toast (synthesized transport failure, no envelope).
 */
export type MockFailureKind = 'forbidden' | 'conflict' | 'invalid_input' | 'network_error';

interface MockClientOptions {
  initial?: Tag[];
  latencyMs?: number;
  failOn?: {
    list?: MockFailureKind;
    create?: MockFailureKind;
    update?: MockFailureKind;
    remove?: MockFailureKind;
  };
}

function delay(ms: number) {
  return new Promise<void>((r) => setTimeout(r, ms));
}

function statusFor(kind: MockFailureKind): number {
  switch (kind) {
    case 'forbidden':
      return 403;
    case 'conflict':
      return 409;
    case 'invalid_input':
      return 400;
    case 'network_error':
      return 0;
  }
}

/** Builds the representative `ApiRequestError` for a mock failure kind. */
function mockFailure(kind: MockFailureKind, op: string, details?: FieldErrorData[]): ApiRequestError {
  const messages: Record<MockFailureKind, string> = {
    forbidden: `You do not have permission to ${op}.`,
    conflict: 'A tag with this purpose and value already exists.',
    invalid_input: 'One or more fields are invalid.',
    network_error: `Could not reach the tags service to ${op}.`,
  };
  return new ApiRequestError(kind, messages[kind], statusFor(kind), details);
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
      if (opts.failOn?.list) throw mockFailure(opts.failOn.list, 'load tags');
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
      if (opts.failOn?.create) {
        const details: FieldErrorData[] | undefined =
          opts.failOn.create === 'invalid_input'
            ? [
                {
                  field: 'value',
                  code: 'tags.value_too_long',
                  message: 'Value must be 512 characters or fewer.',
                },
              ]
            : undefined;
        throw mockFailure(opts.failOn.create, 'create the tag', details);
      }
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
        // The mock has no policy-table concept to consult; default to false.
        // Stories that need to demonstrate exclusion seed `oneOfDomain: true`
        // directly on their `initial` tags instead.
        oneOfDomain: false,
      };
      state.set(uuid, tag);
      return tag;
    },
    async updateColor(uuid: string, color: string | null): Promise<Tag> {
      await delay(latency);
      if (opts.failOn?.update) throw mockFailure(opts.failOn.update, 'update the tag color');
      const existing = state.get(uuid);
      if (!existing) throw new ApiRequestError('not_found', `No tag ${uuid}`, 404);
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
      if (opts.failOn?.update) throw mockFailure(opts.failOn.update, 'update the tag value');
      const existing = state.get(uuid);
      if (!existing) throw new ApiRequestError('not_found', `No tag ${uuid}`, 404);
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
      if (opts.failOn?.remove) throw mockFailure(opts.failOn.remove, 'remove the tag');
      state.delete(uuid);
    },
  };
}
