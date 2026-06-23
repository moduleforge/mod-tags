import { useState, useEffect } from 'react';
import { TagChip } from './TagChip';
import type { Tag } from './lib/api';
import { createTagsClient } from './lib/api';
import { composeColor } from './lib/color';

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
  const [fetchError, setFetchError] = useState<string>('');

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
  const [submitError, setSubmitError] = useState<string>('');

  const purposesKey = JSON.stringify(purposes ?? []);

  async function fetchTags() {
    setLoadState('loading');
    setFetchError('');
    try {
      const fetched = await client.listBySubject(subject, purposes);
      setTags(fetched);
      setLoadState('ready');
      return fetched;
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load tags.';
      setFetchError(msg);
      setLoadState('error');
      return null;
    }
  }

  useEffect(() => {
    let cancelled = false;
    setLoadState('loading');
    setFetchError('');

    client
      .listBySubject(subject, purposes)
      .then((fetched) => {
        if (cancelled) return;
        setTags(fetched);
        setLoadState('ready');
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const msg = err instanceof Error ? err.message : 'Failed to load tags.';
        setFetchError(msg);
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
      // Surface removal errors via the fetch error display area
      const msg = err instanceof Error ? err.message : 'Failed to remove tag.';
      setFetchError(msg);
    }
  }

  async function handleColorChange(tag: Tag, color: string | null) {
    try {
      const updated = await client.updateColor(tag.uuid, color);
      const nextTags = tags.map((t) => (t.uuid === updated.uuid ? updated : t));
      setTags(nextTags);
      onChange?.(nextTags);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update color.';
      setFetchError(msg);
    }
  }

  async function handleValueChange(tag: Tag, value: string) {
    try {
      const updated = await client.updateValue(tag.uuid, value);
      const nextTags = tags.map((t) => (t.uuid === updated.uuid ? updated : t));
      setTags(nextTags);
      onChange?.(nextTags);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update value.';
      setFetchError(msg);
    }
  }

  async function handleAddSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitError('');

    const purposeToSubmit = (() => {
      if (!purposes || purposes.length === 0) return addPurpose.trim();
      if (purposes.length === 1) return purposes[0];
      return addPurpose; // from <select>
    })();

    if (!purposeToSubmit) {
      setSubmitError('Purpose is required.');
      return;
    }
    if (!addValue.trim()) {
      setSubmitError('Value is required.');
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
      // Surface server 409 ("tag already exists") or other errors inline
      const msg = err instanceof Error ? err.message : 'Failed to create tag.';
      setSubmitError(msg);
    } finally {
      setIsSubmitting(false);
    }
  }

  const hasPurposes = purposes && purposes.length > 0;
  const isFixedPurpose = hasPurposes && purposes!.length === 1;
  const isSelectPurpose = hasPurposes && purposes!.length > 1;

  return (
    <div className={`flex flex-col gap-3 ${className ?? ''}`}>
      {/* Existing tags */}
      {loadState === 'loading' || loadState === 'idle' ? (
        <div className="flex flex-wrap gap-1.5" aria-busy="true">
          <span className="h-5 w-20 animate-pulse rounded-full bg-gray-200" />
          <span className="h-5 w-16 animate-pulse rounded-full bg-gray-200" />
        </div>
      ) : loadState === 'error' ? (
        <span className="text-xs text-red-600" role="alert">
          {fetchError}
        </span>
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
              />
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
              >
                <option value="">Select purpose…</option>
                {purposes!.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </select>
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
            />
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

        {submitError && (
          <span className="text-xs text-red-600" role="alert">
            {submitError}
          </span>
        )}
      </form>
    </div>
  );
}
