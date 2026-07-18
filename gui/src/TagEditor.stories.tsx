import type { Story } from '@ladle/react';
import { ToastProvider } from '@moduleforge/core-gui';
import { TagEditor, type TagEditorProps } from './TagEditor';
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

type StoryArgs = Omit<TagEditorProps, 'client' | 'onChange'> & {
  seed: Tag[];
  /** When set, `listBySubject` fails with this `ApiRequestError` kind. */
  failList?: MockFailureKind;
  /** When set, `create` fails with this `ApiRequestError` kind (fill the add form and submit to trigger it). */
  failCreate?: MockFailureKind;
  /** When set, `updateColor`/`updateValue` fail with this `ApiRequestError` kind (edit a chip to trigger it). */
  failUpdate?: MockFailureKind;
};

function Render({ seed, failList, failCreate, failUpdate, ...rest }: StoryArgs) {
  const client = createMockTagsClient({
    initial: seed,
    failOn: {
      list: failList,
      create: failCreate,
      update: failUpdate,
    },
  });
  // `useApiError` (used internally by `TagEditor`) dispatches toast-worthy
  // failures via `useToast`, which requires a `ToastProvider` ancestor — the
  // host app is expected to mount one; the workbench does the same here.
  return (
    <ToastProvider>
      <TagEditor
        {...rest}
        client={client}
        onChange={(tags) => console.log('tags changed:', tags)}
      />
    </ToastProvider>
  );
}

export const Empty: Story<StoryArgs> = (args) => <Render {...args} />;
Empty.args = { subject: SUBJECT, seed: [] };

export const WithExistingTags: Story<StoryArgs> = (args) => <Render {...args} />;
WithExistingTags.args = {
  subject: SUBJECT,
  seed: [
    tag({ uuid: 't1', purpose: 'env', value: 'production', color: '#dc2626' }),
    tag({ uuid: 't2', purpose: 'team', value: 'platform', color: '#2563eb' }),
  ],
};

export const FixedPurpose: Story<StoryArgs> = (args) => <Render {...args} />;
FixedPurpose.args = {
  subject: SUBJECT,
  purposes: ['env'],
  seed: [tag({ uuid: 't1', purpose: 'env', value: 'production', color: '#dc2626' })],
};

export const SelectPurpose: Story<StoryArgs> = (args) => <Render {...args} />;
SelectPurpose.args = {
  subject: SUBJECT,
  purposes: ['env', 'team', 'priority'],
  seed: [
    tag({ uuid: 't1', purpose: 'env', value: 'production', color: '#dc2626' }),
    tag({ uuid: 't2', purpose: 'priority', value: 'p0' }),
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

// Banner surface: a load failure with no field-bound details renders via
// `<ErrorBanner>` in the tag-list area.
export const LoadErrorBanner: Story<StoryArgs> = (args) => <Render {...args} />;
LoadErrorBanner.args = { subject: SUBJECT, seed: [], failList: 'forbidden' };

// Banner surface (mutation path): editing an existing chip's color triggers
// `handleColorChange`, whose failure is routed to the same `<ErrorBanner>` as
// a load failure.
export const MutationErrorBanner: Story<StoryArgs> = (args) => <Render {...args} />;
MutationErrorBanner.args = {
  subject: SUBJECT,
  seed: [tag({ uuid: 't1', purpose: 'env', value: 'production', color: '#dc2626' })],
  failUpdate: 'forbidden',
};

// Field surface: fill in the add form and submit — the `value` input's
// `invalid_input` detail renders inline via `<FieldError>`.
export const CreateFieldError: Story<StoryArgs> = (args) => <Render {...args} />;
CreateFieldError.args = { subject: SUBJECT, seed: [], failCreate: 'invalid_input' };

// Banner surface: fill in the add form and submit — a top-level `conflict`
// ("tag already exists") with no field-bound details renders via
// `<ErrorBanner>` below the form.
export const CreateConflictBanner: Story<StoryArgs> = (args) => <Render {...args} />;
CreateConflictBanner.args = { subject: SUBJECT, seed: [], failCreate: 'conflict' };

// Toast surface: fill in the add form and submit — `network_error` has no
// inline rendering; it is dispatched to the `ToastProvider` mounted above.
export const CreateNetworkErrorToast: Story<StoryArgs> = (args) => <Render {...args} />;
CreateNetworkErrorToast.args = { subject: SUBJECT, seed: [], failCreate: 'network_error' };
