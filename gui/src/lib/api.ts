// API client for the tags service. Zero runtime deps beyond fetch.
//
// Field naming: the wire format uses camelCase (ownerUuid, subjectUuid, createdAt, etc.)
// as confirmed by tags.go tagResponse struct tags. TypeScript types match wire exactly.

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
  /** Fetch implementation; defaults to global fetch. */
  fetchImpl?: typeof fetch;
  /**
   * Called before every request; return an object of headers to merge.
   * Typically used by consumers to inject Authorization: Bearer <token>.
   */
  headers?: () => Record<string, string> | Promise<Record<string, string>>;
}

interface ErrorResponse {
  error?: string;
  message?: string;
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (res.ok) {
    // 204 No Content has no body
    if (res.status === 204) {
      return undefined as T;
    }
    return res.json() as Promise<T>;
  }

  // Attempt to parse error body as JSON { error: "..." } or { message: "..." }
  let message = res.statusText;
  try {
    const body = (await res.json()) as ErrorResponse;
    message = body.error ?? body.message ?? message;
  } catch {
    // Body wasn't JSON; use status text
  }
  throw new Error(message);
}

export function createTagsClient(opts: TagsClientOptions): {
  listBySubject(subjectUuid: string, purposes?: string[]): Promise<Tag[]>;
  create(input: { subject: string; purpose: string; value: string; color?: string }): Promise<Tag>;
  updateColor(uuid: string, color: string | null): Promise<Tag>;
  updateValue(uuid: string, value: string): Promise<Tag>;
  remove(uuid: string): Promise<void>;
} {
  const fetchFn = opts.fetchImpl ?? globalThis.fetch;

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

      const res = await fetchFn(url, { headers });
      // Server returns { tags: Tag[] }
      const body = await handleResponse<{ tags: Tag[] }>(res);
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
      const res = await fetchFn(`${opts.baseUrl}/tags`, {
        method: 'POST',
        headers,
        body: JSON.stringify(input),
      });
      return handleResponse<Tag>(res);
    },

    async updateColor(uuid: string, color: string | null): Promise<Tag> {
      const headers = await buildHeaders();
      const res = await fetchFn(`${opts.baseUrl}/tags/${uuid}`, {
        method: 'PUT',
        headers,
        body: JSON.stringify({ color }),
      });
      return handleResponse<Tag>(res);
    },

    async updateValue(uuid: string, value: string): Promise<Tag> {
      const headers = await buildHeaders();
      const res = await fetchFn(`${opts.baseUrl}/tags/${uuid}`, {
        method: 'PATCH',
        headers,
        body: JSON.stringify({ value }),
      });
      return handleResponse<Tag>(res);
    },

    async remove(uuid: string): Promise<void> {
      const headers = await buildHeaders();
      const res = await fetchFn(`${opts.baseUrl}/tags/${uuid}`, {
        method: 'DELETE',
        headers,
      });
      return handleResponse<void>(res);
    },
  };
}
