import type { Story } from '@ladle/react';
import { ToastProvider } from '@moduleforge/core-gui';
import { TagList, type TagListProps } from './TagList';
import type { Tag } from './lib/api';
import { createMockTagsClient, type MockFailureKind } from './lib/mockClient';

const SUBJECT = 'subject-1';

function tag(partial: Partial<Tag> & Pick<Tag, 'uuid' | 'purpose' | 'value'>): Tag {
  return {
    ownerUuid: 'mock-owner',
    subjectUuid: SUBJECT,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...partial,
  };
}

type StoryArgs = Omit<TagListProps, 'client'> & {
  seed: Tag[];
  /** When set, `listBySubject` fails with this `ApiRequestError` kind. */
  failList?: MockFailureKind;
};

function Render({ seed, failList, ...rest }: StoryArgs) {
  const client = createMockTagsClient({
    initial: seed,
    failOn: failList ? { list: failList } : undefined,
  });
  // `useApiError` (used internally by `TagList`) dispatches toast-worthy
  // failures via `useToast`, which requires a `ToastProvider` ancestor — the
  // host app is expected to mount one; the workbench does the same here.
  return (
    <ToastProvider>
      <TagList {...rest} client={client} />
    </ToastProvider>
  );
}

export const Empty: Story<StoryArgs> = (args) => <Render {...args} />;
Empty.args = { subject: SUBJECT, seed: [] };

export const Few: Story<StoryArgs> = (args) => <Render {...args} />;
Few.args = {
  subject: SUBJECT,
  seed: [
    tag({ uuid: 't1', purpose: 'env', value: 'production', color: '#dc2626' }),
    tag({ uuid: 't2', purpose: 'team', value: 'platform', color: '#2563eb' }),
  ],
};

export const Many: Story<StoryArgs> = (args) => <Render {...args} />;
Many.args = {
  subject: SUBJECT,
  seed: [
    tag({ uuid: 't1', purpose: 'env', value: 'production', color: '#dc2626' }),
    tag({ uuid: 't2', purpose: 'env', value: 'staging', color: '#059669' }),
    tag({ uuid: 't3', purpose: 'env', value: 'dev' }),
    tag({ uuid: 't4', purpose: 'team', value: 'platform', color: '#2563eb' }),
    tag({ uuid: 't5', purpose: 'team', value: 'growth', color: '#7c3aed' }),
    tag({ uuid: 't6', purpose: 'priority', value: 'p0', color: '#f97316' }),
    tag({ uuid: 't7', purpose: 'priority', value: 'p1' }),
    tag({ uuid: 't8', purpose: 'region', value: 'us-east', color: '#fde68a' }),
  ],
};

export const FilteredByPurpose: Story<StoryArgs> = (args) => <Render {...args} />;
FilteredByPurpose.args = {
  subject: SUBJECT,
  purposes: ['env'],
  seed: [
    tag({ uuid: 't1', purpose: 'env', value: 'production', color: '#dc2626' }),
    tag({ uuid: 't2', purpose: 'team', value: 'platform', color: '#2563eb' }),
    tag({ uuid: 't3', purpose: 'env', value: 'staging', color: '#059669' }),
  ],
};

export const NoPurposeDisplay: Story<StoryArgs> = (args) => <Render {...args} />;
NoPurposeDisplay.args = {
  subject: SUBJECT,
  noPurpose: true,
  seed: [
    tag({ uuid: 't1', purpose: 'label', value: 'beta' }),
    tag({ uuid: 't2', purpose: 'label', value: 'alpha' }),
  ],
};

// Banner surface: a top-level `forbidden`/`conflict` error with no
// field-bound details renders via `<ErrorBanner>`.
export const LoadErrorBanner: Story<StoryArgs> = (args) => <Render {...args} />;
LoadErrorBanner.args = { subject: SUBJECT, seed: [], failList: 'forbidden' };

// Toast surface: `network_error` has no inline rendering — it is dispatched
// to the `ToastProvider` mounted above, and the load area shows nothing.
export const LoadErrorToast: Story<StoryArgs> = (args) => <Render {...args} />;
LoadErrorToast.args = { subject: SUBJECT, seed: [], failList: 'network_error' };
