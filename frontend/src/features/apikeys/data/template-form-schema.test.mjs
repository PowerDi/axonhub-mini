import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { formSchemaFactory } from './template-form-schema.ts';

const schema = formSchemaFactory((key) => key);

for (const mode of ['include', 'exclude']) {
  test(`template form preserves the ${mode} channel matching mode in its submit payload`, () => {
    const values = schema.parse({
      name: 'Template',
      profile: {
        name: 'Template',
        modelMappings: [],
        channelIDs: [1, 2],
        channelIDsMatchMode: mode,
        channelTags: ['production'],
        channelTagsMatchMode: 'none',
      },
    });

    assert.equal(values.profile.channelIDsMatchMode, mode);
    assert.deepEqual(values.profile.channelIDs, [1, 2]);
    assert.equal(values.profile.channelTagsMatchMode, 'none');
  });
}

for (const kind of ['create', 'edit']) {
  test(`${kind} template dialog exposes both channel matching modes`, () => {
    const source = readFileSync(new URL(`../components/apikeys-${kind}-template-dialog.tsx`, import.meta.url), 'utf8');

    assert.match(source, /name='profile\.channelIDsMatchMode'/);
    assert.match(source, /<SelectItem value='include'>/);
    assert.match(source, /<SelectItem value='exclude'>/);
    assert.match(source, /form\.watch\('profile\.channelIDsMatchMode'\)/);
    if (kind === 'edit') {
      assert.match(source, /channelIDsMatchMode: profile\?\.channelIDsMatchMode \?\? 'include'/);
    } else {
      assert.match(source, /channelIDsMatchMode: 'include'/);
    }
  });
}
