import { ImportIdMapEntry } from '../../types/dataExchange';

interface IDMappingTableProps {
  mappings: ImportIdMapEntry[];
  title: string;
}

export function IDMappingTable({ mappings, title }: IDMappingTableProps) {
  if (mappings.length === 0) return null;

  return (
    <section className="preview-id-map" aria-label={title}>
      <h3>{title} ({mappings.length})</h3>
      <div className="validation-table-scroll">
        <table className="validation-table" aria-label={title}>
          <thead>
            <tr>
              <th scope="col">Source ID</th>
              <th scope="col">New Entity ID</th>
            </tr>
          </thead>
          <tbody>
            {mappings.map(({ source_id, entity_id }) => (
              <tr key={`${source_id}:${entity_id}`}>
                <td><code>{source_id}</code></td>
                <td><code>{entity_id}</code></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
