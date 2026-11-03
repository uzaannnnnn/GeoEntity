import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { ImportPreviewSummary } from './ImportPreviewSummary';
import { ImportPreview } from '../../types/dataExchange';

const preview: ImportPreview = {
  format: 'csv',
  total_rows: 1,
  valid_rows: 1,
  invalid_rows: 0,
  warnings: [],
  errors: [],
  errors_truncated: false,
  warnings_truncated: false,
  duplicate_candidates_truncated: false,
  existing_ids_truncated: false,
  new_catalog_items_truncated: false,
  duplicate_candidates: [],
  existing_ids: [],
  new_catalog_items: [
    { kind: 'tag', name: 'Fleet' },
    { kind: 'category', name: 'Operations' },
  ],
  preview_sha256: 'sha256',
  id_mode: 'regenerate',
  id_map: [{ source_id: 'source-uuid', entity_id: 'new-uuid' }],
};

describe('ImportPreviewSummary', () => {
  it('renders backend singular catalog kinds as the matching tag and category', () => {
    const markup = renderToStaticMarkup(<ImportPreviewSummary preview={preview} />);

    expect(markup).toContain('pill-tags');
    expect(markup).toContain('Tag:');
    expect(markup).toContain('Fleet');
    expect(markup).toContain('pill-categories');
    expect(markup).toContain('Category:');
    expect(markup).toContain('Operations');
  });

  it('shows the source-to-generated ID mapping before confirmation', () => {
    const markup = renderToStaticMarkup(<ImportPreviewSummary preview={preview} />);

    expect(markup).toContain('Preview ID Mapping');
    expect(markup).toContain('Source ID');
    expect(markup).toContain('New Entity ID');
    expect(markup).toContain('source-uuid');
    expect(markup).toContain('new-uuid');
  });
});
