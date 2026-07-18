// API client for the tags service.
//
// Field naming: the wire format uses camelCase (ownerUuid, subjectUuid, createdAt, etc.)
// as confirmed by tags.go tagResponse struct tags. TypeScript types match wire exactly.
//
// Transport/error handling is delegated to `@moduleforge/core-gui`'s shared `request()`
// wrapper (docs/mf-standards/architecture/api-response-design.md, "GUI-facing error-data
// contract" / "Client contract (ApiRequestError)"): it owns method/headers/body, JSON
// parsing, the 204-no-body case, the nested `{error:{code,message,details?}}` envelope
// parse, the synthesized `network_error`/status-0 transport failure, and the 401-redirect
// special case. The wire error types and `ApiRequestError` are re-exported here (not
// redefined) so this module stays the single source of truth `@moduleforge/tags-gui`
// consumers import from.

import type { ApiError, ApiErrorResponse, FieldErrorData } from '@moduleforge/core-gui';
import { ApiRequestError, request } from '@moduleforge/core-gui';

export type { ApiError, ApiErrorResponse, FieldErrorData };
export { ApiRequestError };

export interface Tag {
  uuid: string;
  ownerUuid: string;
  subjectUuid: string;
  purpose: string;
  value: string;
  color?: string;
  createdAt: string;
  updatedAt: string;
}

export interface TagsClientOptions {
  /** Base URL for the API (e.g. "/v1"). Required. */
  baseUrl: string;
  /**
   * Called before every request; return an object of headers to merge.
   * Typically used by consumers to inject Authorization: Bearer <token>.
   */
  headers?: () => Record<string, string> | Promise<Record<string, string>>;
}

export function createTagsClient(opts: TagsClientOptions): {
  listBySubject(subjectUuid: string, purposes?: string[]): Promise<Tag[]>;
  create(input: { subject: string; purpose: string; value: string; color?: string }): Promise<Tag>;
  updateColor(uuid: string, color: string | null): Promise<Tag>;
  updateValue(uuid: string, value: string): Promise<Tag>;
  remove(uuid: string): Promise<void>;
} {
  async function buildHeaders(): Promise<Record<string, string>> {
    const extra = opts.headers ? await opts.headers() : {};
    return { 'Content-Type': 'application/json', ...extra };
  }

  return {
    async listBySubject(subjectUuid: string, purposes?: string[]): Promise<Tag[]> {
      const headers = await buildHeaders();
      const base = `${opts.baseUrl}/entities/${subjectUuid}/tags`;

      let url: string;
      if (purposes && purposes.length === 1) {
        // Delegate single-purpose filtering to the server to avoid over-fetching.
        url = `${base}?purpose=${encodeURIComponent(purposes[0])}`;
      } else {
        url = base;
      }

      // Server returns { tags: Tag[] }. The list-envelope shape is not changing in this
      // plan, so keep extracting `.tags` rather than assuming a bare array.
      const body = await request<{ tags: Tag[] }>(url, { headers });
      const tags = body.tags ?? [];

      // purposes=undefined or purposes=[] means "all" — no filter applied.
      // Single-purpose case is already filtered by the server.
      if (!purposes || purposes.length <= 1) {
        return tags;
      }
      // Multiple purposes: filter client-side
      const purposeSet = new Set(purposes);
      return tags.filter((t) => purposeSet.has(t.purpose));
    },

    async create(input: {
      subject: string;
      purpose: string;
      value: string;
      color?: string;
    }): Promise<Tag> {
      const headers = await buildHeaders();
      return request<Tag>(`${opts.baseUrl}/tags`, {
        method: 'POST',
        headers,
        body: JSON.stringify(input),
      });
    },

    async updateColor(uuid: string, color: string | null): Promise<Tag> {
      const headers = await buildHeaders();
      return request<Tag>(`${opts.baseUrl}/tags/${uuid}`, {
        method: 'PUT',
        headers,
        body: JSON.stringify({ color }),
      });
    },

    async updateValue(uuid: string, value: string): Promise<Tag> {
      const headers = await buildHeaders();
      return request<Tag>(`${opts.baseUrl}/tags/${uuid}`, {
        method: 'PATCH',
        headers,
        body: JSON.stringify({ value }),
      });
    },

    async remove(uuid: string): Promise<void> {
      const headers = await buildHeaders();
      return request<void>(`${opts.baseUrl}/tags/${uuid}`, {
        method: 'DELETE',
        headers,
      });
    },
  };
}
