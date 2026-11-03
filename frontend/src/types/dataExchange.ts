export type ImportFormat = 'csv' | 'geojson';
export type ImportIdMode = 'preserve' | 'regenerate';

export interface ImportIssue {
  row?: number;
  feature?: number;
  field?: string;
  code: string;
  message: string;
}

export interface ImportWarning {
  row?: number;
  feature?: number;
  field?: string;
  code: string;
  message: string;
}

export interface ImportDuplicateCandidate {
  row?: number;
  feature?: number;
  entity_id: string;
  reason: string;
}

export interface ImportCatalogItem {
  kind: 'tag' | 'category';
  name: string;
}

export interface ImportIdMapEntry {
  source_id: string;
  entity_id: string;
}

export interface ImportPreview {
  format: ImportFormat;
  total_rows: number;
  valid_rows: number;
  invalid_rows: number;
  warnings: ImportWarning[];
  errors: ImportIssue[];
  errors_truncated: boolean;
  warnings_truncated: boolean;
  duplicate_candidates_truncated: boolean;
  existing_ids_truncated: boolean;
  new_catalog_items_truncated: boolean;
  duplicate_candidates: ImportDuplicateCandidate[];
  existing_ids: string[];
  new_catalog_items: ImportCatalogItem[];
  preview_sha256: string;
  id_mode: ImportIdMode;
  id_map: ImportIdMapEntry[];
}

export interface ImportResult {
  batch_id: string;
  format: ImportFormat;
  imported: number;
  created: number;
  tags_created: number;
  categories_created: number;
  id_map: ImportIdMapEntry[];
}

export interface ExportFilters {
  q?: string;
  type?: string;
  status?: string;
  spreadsheet_safe?: boolean;
}
