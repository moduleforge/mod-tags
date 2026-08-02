import type { Story } from '@ladle/react';
import { TagChip, type TagChipProps } from './TagChip';
import type { Tag } from './lib/api';

function makeTag(partial: Partial<Tag>): Tag {
  return {
    uuid: partial.uuid ?? 'mock-1',
    ownerUuid: 'mock-owner',
    subjectUuid: 'mock-subject',
    purpose: partial.purpose ?? 'env',
    value: partial.value ?? 'production',
    color: partial.color,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    oneOfDomain: partial.oneOfDomain ?? false,
  };
}

export const Default: Story<TagChipProps> = (args) => <TagChip {...args} />;
Default.args = {
  tag: makeTag({ purpose: 'env', value: 'production' }),
};

export const Colored: Story<TagChipProps> = (args) => <TagChip {...args} />;
Colored.args = {
  tag: makeTag({ purpose: 'team', value: 'platform', color: '#2563eb' }),
};

export const ColoredLight: Story<TagChipProps> = (args) => <TagChip {...args} />;
ColoredLight.args = {
  tag: makeTag({ purpose: 'priority', value: 'low', color: '#fde68a' }),
};

export const NoPurpose: Story<TagChipProps> = (args) => <TagChip {...args} />;
NoPurpose.args = {
  tag: makeTag({ purpose: 'label', value: 'beta' }),
  noPurpose: true,
};

export const Removable: Story<TagChipProps> = (args) => <TagChip {...args} />;
Removable.args = {
  tag: makeTag({ purpose: 'env', value: 'production', color: '#dc2626' }),
  onRemove: () => console.log('remove'),
};

export const ColorEditable: Story<TagChipProps> = (args) => <TagChip {...args} />;
ColorEditable.args = {
  tag: makeTag({ purpose: 'team', value: 'platform', color: '#2563eb' }),
  onColorChange: (color) => console.log('color change:', color),
};

export const ValueEditable: Story<TagChipProps> = (args) => <TagChip {...args} />;
ValueEditable.args = {
  tag: makeTag({ purpose: 'env', value: 'production' }),
  onValueChange: (value) => console.log('value change:', value),
};

export const FullyInteractive: Story<TagChipProps> = (args) => <TagChip {...args} />;
FullyInteractive.args = {
  tag: makeTag({ purpose: 'env', value: 'staging', color: '#059669' }),
  onRemove: () => console.log('remove'),
  onColorChange: (color) => console.log('color change:', color),
  onValueChange: (value) => console.log('value change:', value),
};
