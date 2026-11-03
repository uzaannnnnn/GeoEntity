import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { MonitoringDashboard, deviceFreshness } from './MonitoringDashboard';

describe('MonitoringDashboard',()=>{
  it('renders a useful empty state when no entities are available',()=>{const html=renderToStaticMarkup(<MonitoringDashboard entities={[]} onClose={()=>undefined}/>);expect(html).toContain('Monitoring');expect(html).toContain('No entities are available.')});
  it('shows freshness as unknown without cadence and stale after three expected intervals',()=>{expect(deviceFreshness(null,60,100000)).toBe('Unknown');expect(deviceFreshness(new Date(0).toISOString(),60,179999)).toBe('Fresh');expect(deviceFreshness(new Date(0).toISOString(),60,180001)).toBe('Stale')});
});
