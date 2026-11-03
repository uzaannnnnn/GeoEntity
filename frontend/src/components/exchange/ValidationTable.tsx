import { ImportFormat, ImportIssue, ImportWarning } from '../../types/dataExchange';
import { AlertIcon, InfoIcon } from '../Icons';

interface ValidationTableProps {
  format: ImportFormat;
  errors: ImportIssue[];
  warnings: ImportWarning[];
  errorsTruncated?: boolean;
  warningsTruncated?: boolean;
}

export function ValidationTable({
  format,
  errors,
  warnings,
  errorsTruncated = false,
  warningsTruncated = false,
}: ValidationTableProps) {
  const hasErrors = errors.length > 0;
  const hasWarnings = warnings.length > 0;

  if (!hasErrors && !hasWarnings) {
    return null;
  }

  const locationHeader = format === 'csv' ? 'Row' : 'Feature';

  return (
    <div className="validation-table-wrapper">
      {hasErrors && (
        <div className="validation-section errors-section">
          <div className="validation-section-header">
            <span className="validation-title error-title">
              <AlertIcon size={16} />
              Validation Errors ({errors.length}
              {errorsTruncated ? '+' : ''})
            </span>
            {errorsTruncated && (
              <span className="truncation-badge">First 100 errors shown</span>
            )}
          </div>

          <div className="validation-table-scroll">
            <table className="validation-table" aria-label="Validation error details">
              <thead>
                <tr>
                  <th scope="col" style={{ width: '80px' }}>
                    {locationHeader}
                  </th>
                  <th scope="col" style={{ width: '130px' }}>
                    Field / Property
                  </th>
                  <th scope="col" style={{ width: '140px' }}>
                    Code
                  </th>
                  <th scope="col">Message</th>
                </tr>
              </thead>
              <tbody>
                {errors.map((err, idx) => {
                  const loc = format === 'csv' ? err.row : err.feature;
                  return (
                    <tr key={`err-${idx}`} className="error-row">
                      <td className="loc-cell">
                        {loc !== undefined ? `${locationHeader} ${loc}` : '—'}
                      </td>
                      <td className="field-cell">
                        {err.field ? <code>{err.field}</code> : '—'}
                      </td>
                      <td className="code-cell">
                        <span className="error-code-badge">{err.code}</span>
                      </td>
                      <td className="msg-cell">{err.message}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {hasWarnings && (
        <div className="validation-section warnings-section">
          <div className="validation-section-header">
            <span className="validation-title warning-title">
              <InfoIcon size={16} />
              Validation Notices & Warnings ({warnings.length}
              {warningsTruncated ? '+' : ''})
            </span>
            {warningsTruncated && (
              <span className="truncation-badge">First 100 shown</span>
            )}
          </div>

          <div className="validation-table-scroll">
            <table className="validation-table" aria-label="Validation warnings and notices">
              <thead>
                <tr>
                  <th scope="col" style={{ width: '80px' }}>
                    {locationHeader}
                  </th>
                  <th scope="col" style={{ width: '130px' }}>
                    Field / Target
                  </th>
                  <th scope="col" style={{ width: '140px' }}>
                    Code
                  </th>
                  <th scope="col">Message</th>
                </tr>
              </thead>
              <tbody>
                {warnings.map((w, idx) => {
                  const loc = format === 'csv' ? w.row : w.feature;
                  return (
                    <tr key={`warn-${idx}`} className="warning-row">
                      <td className="loc-cell">
                        {loc !== undefined && loc > 0 ? `${locationHeader} ${loc}` : '—'}
                      </td>
                      <td className="field-cell">
                        {w.field ? <code>{w.field}</code> : '—'}
                      </td>
                      <td className="code-cell">
                        <span className="warning-code-badge">{w.code}</span>
                      </td>
                      <td className="msg-cell">{w.message}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
