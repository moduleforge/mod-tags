import { useState, useEffect } from 'react';
import { ErrorBanner, FieldError, useApiError, type FieldErrorData } from '@moduleforge/core-gui';
import { TagChip } from './TagChip';
import type { Tag } from './lib/api';
import { createTagsClient, ApiRequestError } from './lib/api';
import { composeColor } from './lib/color';

/**
 * REQUIRES a `@moduleforge/core-gui` `<ToastProvider>` ancestor. `TagEditor`
 * calls `useApiError` unconditionally on every render (not only when an
 * error occurs), and `useApiError` in turn calls `core-gui`'s `useToast()`
 * unconditionally — which throws if no `<ToastProvider>` is mounted above it
 * in the tree. Host applications must wrap `TagEditor` (or an ancestor of
 * it) in `<ToastProvider>`, even if no API errors are expected.
 */
export interface TagEditorProps {
  /** Subject entity UUID */
  subject: string;
  /**
   * Restrict to these purposes. undefined and [] are equivalent — both mean
   * "all purposes / free-form". Enforced via `purposes?: string[]` and
   * runtime check on `.length > 0`.
   */
  purposes?: string[];
  noPurpose?: boolean;
  client: ReturnType<typeof createTagsClient>;
  className?: string;
  /** Called after each successful mutation with the updated tag list */
  onChange?: (tags: Tag[]) => void;
}

type LoadState = 'idle' | 'loading' | 'error' | 'ready';

/** Field names the add-form renders; binds `useApiError`'s field-level details to them. */
const ADD_FORM_FIELDS = ['purpose', 'value'] as const;
/**
 * Same as `ADD_FORM_FIELDS`, but for fixed-purpose mode, where no `purpose`
 * input/select is rendered — used so an unexpected `purpose`-field error
 * falls through to the banner instead of being silently dropped.
 */
const SUBMIT_FIELDS_NO_PURPOSE = ['value'] as const;

/**
 * Wraps a caught value as an `ApiRequestError`. Every failure `client`
 * surfaces already comes through `@moduleforge/core-gui`'s `request()` as an
 * `ApiRequestError` (task 001); the fallback here is defensive only, in case
 * a caller-supplied `client` implementation (e.g. a test double) throws
 * something else.
 */
function toApiRequestError(err: unknown, fallbackMessage: string): ApiRequestError {
  return err instanceof ApiRequestError ? err : new ApiRequestError('internal_error', fallbackMessage, 0);
}

export function TagEditor({
  subject,
  purposes,
  noPurpose,
  client,
  className,
  onChange,
}: TagEditorProps) {
  const [tags, setTags] = useState<Tag[]>([]);
  const [loadState, setLoadState] = useState<LoadState>('idle');
  // Load *and* mutation (remove/color/value) failures share this banner — all
  // originate from operating on the existing tag list, none bind to the
  // add-form's inputs.
  const [loadError, setLoadError] = useState<ApiRequestError | null>(null);

  // Add-form state
  const [addPurpose, setAddPurpose] = useState<string>(() => {
    if (purposes && purposes.length === 1) return purposes[0];
    return '';
  });
  const [addValue, setAddValue] = useState('');
  const [addRgb, setAddRgb] = useState('#e5e7eb');
  const [addAlpha, setAddAlpha] = useState(255);
  const [includeColor, setIncludeColor] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<ApiRequestError | null>(null);
  // Local pre-submit validation ("Purpose is required." / "Value is
  // required."), synthesized in the same `FieldErrorData` shape as a
  // server-sent detail so it renders through the same `<FieldError>` slot
  // instead of separate ad-hoc markup.
  const [preSubmitFieldError, setPreSubmitFieldError] = useState<FieldErrorData | null>(null);

  const purposesKey = JSON.stringify(purposes ?? []);

  // In fixed-purpose mode (a single, pre-selected purpose), the add-form
  // renders no `purpose` input/select/<FieldError> — only a static <span> —
  // so 'purpose' must be omitted here. Otherwise a server-sent `purpose`
  // field detail would be treated as field-bound by `useApiError` and
  // silently dropped (no field widget renders it, and it wouldn't fall
  // through to the banner either).
  const hasPurposes = purposes && purposes.length > 0;
  const isFixedPurpose = hasPurposes && purposes!.length === 1;
  const submitFields = isFixedPurpose ? SUBMIT_FIELDS_NO_PURPOSE : ADD_FORM_FIELDS;

  const { bannerError: loadBannerError } = useApiError(loadError);
  const { fieldErrors: submitFieldErrors, bannerError: submitBannerError } = useApiError(submitError, {
    fields: submitFields,
  });

  const purposeFieldError =
    preSubmitFieldError?.field === 'purpose' ? preSubmitFieldError : (submitFieldErrors.purpose ?? null);
  const valueFieldError =
    preSubmitFieldError?.field === 'value' ? preSubmitFieldError : (submitFieldErrors.value ?? null);

  async function fetchTags() {
    setLoadState('loading');
    setLoadError(null);
    try {
      const fetched = await client.listBySubject(subject, purposes);
      setTags(fetched);
      setLoadState('ready');
      return fetched;
    } catch (err: unknown) {
      setLoadError(toApiRequestError(err, 'Failed to load tags.'));
      setLoadState('error');
      return null;
    }
  }

  useEffect(() => {
    let cancelled = false;
    setLoadState('loading');
    setLoadError(null);

    client
      .listBySubject(subject, purposes)
      .then((fetched) => {
        if (cancelled) return;
        setTags(fetched);
        setLoadState('ready');
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoadError(toApiRequestError(err, 'Failed to load tags.'));
        setLoadState('error');
      });

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [subject, purposesKey]);

  // Reset the fixed purpose if purposes changes
  useEffect(() => {
    if (purposes && purposes.length === 1) {
      setAddPurpose(purposes[0]);
    } else if (!purposes || purposes.length === 0) {
      setAddPurpose('');
    }
  }, [purposesKey]); // eslint-disable-line react-hooks/exhaustive-deps

  async function handleRemove(tag: Tag) {
    try {
      await client.remove(tag.uuid);
      const updated = await fetchTags();
      if (updated) onChange?.(updated);
    } catch (err: unknown) {
      // Surface removal errors via the shared load/mutation error banner
      setLoadError(toApiRequestError(err, 'Failed to remove tag.'));
    }
  }

  async function handleColorChange(tag: Tag, color: string | null) {
    try {
      const updated = await client.updateColor(tag.uuid, color);
      const nextTags = tags.map((t) => (t.uuid === updated.uuid ? updated : t));
      setTags(nextTags);
      onChange?.(nextTags);
    } catch (err: unknown) {
      setLoadError(toApiRequestError(err, 'Failed to update color.'));
    }
  }

  async function handleValueChange(tag: Tag, value: string) {
    try {
      const updated = await client.updateValue(tag.uuid, value);
      const nextTags = tags.map((t) => (t.uuid === updated.uuid ? updated : t));
      setTags(nextTags);
      onChange?.(nextTags);
    } catch (err: unknown) {
      setLoadError(toApiRequestError(err, 'Failed to update value.'));
    }
  }

  async function handleAddSubmit(e: React.FormEvent) {
    e.preventDefault();
    setPreSubmitFieldError(null);
    setSubmitError(null);

    const purposeToSubmit = (() => {
      if (!purposes || purposes.length === 0) return addPurpose.trim();
      if (purposes.length === 1) return purposes[0];
      return addPurpose; // from <select>
    })();

    if (!purposeToSubmit) {
      setPreSubmitFieldError({ field: 'purpose', code: 'client.required', message: 'Purpose is required.' });
      return;
    }
    if (!addValue.trim()) {
      setPreSubmitFieldError({ field: 'value', code: 'client.required', message: 'Value is required.' });
      return;
    }

    setIsSubmitting(true);
    try {
      await client.create({
        subject,
        purpose: purposeToSubmit,
        value: addValue.trim(),
        color: includeColor ? composeColor(addRgb, addAlpha) : undefined,
      });
      // Reset form
      if (!purposes || purposes.length !== 1) setAddPurpose('');
      setAddValue('');
      setIncludeColor(false);
      setAddRgb('#e5e7eb');
      setAddAlpha(255);

      const updated = await fetchTags();
      if (updated) onChange?.(updated);
    } catch (err: unknown) {
      // Surface a 409 ("tag already exists"), invalid_input field details, or
      // any other create failure via useApiError's field/banner/toast routing
      setSubmitError(toApiRequestError(err, 'Failed to create tag.'));
    } finally {
      setIsSubmitting(false);
    }
  }

  const isSelectPurpose = hasPurposes && purposes!.length > 1;

  return (
    <div className={`flex flex-col gap-3 ${className ?? ''}`}>
      {/* Existing tags */}
      <div className="flex flex-col gap-1.5">
        {loadState === 'loading' || loadState === 'idle' ? (
          <div className="flex flex-wrap gap-1.5" aria-busy="true">
            <span className="h-5 w-20 animate-pulse rounded-full bg-gray-200" />
            <span className="h-5 w-16 animate-pulse rounded-full bg-gray-200" />
          </div>
        ) : tags.length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {tags.map((tag) => (
              <TagChip
                key={tag.uuid}
                tag={tag}
                noPurpose={noPurpose}
                onRemove={() => void handleRemove(tag)}
                onColorChange={(color) => void handleColorChange(tag, color)}
                onValueChange={(value) => void handleValueChange(tag, value)}
              />
            ))}
          </div>
        ) : null}
        <ErrorBanner error={loadBannerError} />
      </div>

      {/* Add form */}
      <form onSubmit={(e) => void handleAddSubmit(e)} className="flex flex-col gap-2">
        <div className="flex flex-wrap items-end gap-2">
          {/* Purpose input: free-form, fixed-label, or select */}
          {!isFixedPurpose && !isSelectPurpose && (
            <div className="flex flex-col gap-1">
              <label className="text-xs text-gray-600" htmlFor={`add-purpose-${subject}`}>
                Purpose
              </label>
              <input
                id={`add-purpose-${subject}`}
                type="text"
                className="h-8 rounded border border-gray-300 px-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-400"
                placeholder="purpose"
                value={addPurpose}
                onChange={(e) => setAddPurpose(e.target.value)}
                maxLength={128}
                aria-describedby={purposeFieldError ? `add-purpose-error-${subject}` : undefined}
              />
              <FieldError error={purposeFieldError} id={`add-purpose-error-${subject}`} />
            </div>
          )}

          {isFixedPurpose && (
            <div className="flex flex-col gap-1">
              <span className="text-xs text-gray-600">Purpose</span>
              <span className="inline-flex h-8 items-center rounded border border-gray-200 bg-gray-50 px-2 text-sm text-gray-700">
                {purposes![0]}
              </span>
            </div>
          )}

          {isSelectPurpose && (
            <div className="flex flex-col gap-1">
              <label className="text-xs text-gray-600" htmlFor={`add-purpose-sel-${subject}`}>
                Purpose
              </label>
              <select
                id={`add-purpose-sel-${subject}`}
                className="h-8 rounded border border-gray-300 px-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-400"
                value={addPurpose}
                onChange={(e) => setAddPurpose(e.target.value)}
                aria-describedby={purposeFieldError ? `add-purpose-error-${subject}` : undefined}
              >
                <option value="">Select purpose…</option>
                {purposes!.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </select>
              <FieldError error={purposeFieldError} id={`add-purpose-error-${subject}`} />
            </div>
          )}

          {/* Value input — always shown */}
          <div className="flex flex-col gap-1">
            <label className="text-xs text-gray-600" htmlFor={`add-value-${subject}`}>
              Value
            </label>
            <input
              id={`add-value-${subject}`}
              type="text"
              className="h-8 rounded border border-gray-300 px-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-400"
              placeholder="value"
              value={addValue}
              onChange={(e) => setAddValue(e.target.value)}
              required
              maxLength={512}
              aria-describedby={valueFieldError ? `add-value-error-${subject}` : undefined}
            />
            <FieldError error={valueFieldError} id={`add-value-error-${subject}`} />
          </div>

          {/* Color toggle + picker */}
          <div className="flex flex-col gap-1">
            <label className="text-xs text-gray-600">Color</label>
            <div className="flex h-8 items-center gap-1.5">
              <input
                type="checkbox"
                id={`add-color-toggle-${subject}`}
                checked={includeColor}
                onChange={(e) => setIncludeColor(e.target.checked)}
                className="cursor-pointer"
              />
              {includeColor && (
                <>
                  <input
                    type="color"
                    value={addRgb}
                    onChange={(e) => setAddRgb(e.target.value)}
                    className="h-6 w-10 cursor-pointer rounded border-0 p-0"
                    aria-label="Tag color"
                  />
                  <input
                    type="range"
                    min={0}
                    max={255}
                    value={addAlpha}
                    onChange={(e) => setAddAlpha(Number(e.target.value))}
                    className="w-20"
                    aria-label="Tag color alpha"
                  />
                </>
              )}
            </div>
          </div>

          <button
            type="submit"
            disabled={isSubmitting}
            className="h-8 rounded bg-gray-900 px-3 text-sm text-white hover:bg-gray-700 disabled:opacity-50"
          >
            {isSubmitting ? 'Adding…' : 'Add'}
          </button>
        </div>

        <ErrorBanner error={submitBannerError} />
      </form>
    </div>
  );
}
