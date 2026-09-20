import { APPLICATION_TYPES } from '@/constants/applicationTypes';
import { Navigate, useSearchParams } from 'react-router-dom';
import { useI18n } from '@/i18n';
import {
  getApplicationList,
  getDeviceList,
  getEdgeList,
  getTrafficMetricsList,
} from '@/services/api';
import {
  Activity,
  Cable,
  HardDrive,
  type LucideIcon,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import './index.less';

type DistributionItem = { type: string; value: number };
type TrafficPoint = { time: Date; application: string; value: number };

const chartColors = ['rgb(var(--chart-1))', 'rgb(var(--chart-2))', 'rgb(var(--chart-3))', 'rgb(var(--chart-4))', 'rgb(var(--chart-5))', 'rgb(var(--chart-6))'];

const applicationLabels = Object.fromEntries(
  APPLICATION_TYPES.map((type) => [type.value, type.label]),
);

const numberValue = (value: number | string | undefined) => {
  const parsed = Number(value || 0);
  return Number.isFinite(parsed) ? parsed : 0;
};

const formatChartTime = (date: Date) =>
  `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;

const formatTraffic = (bitsPerSecond: number) => {
  if (bitsPerSecond >= 1_000_000_000)
    return `${(bitsPerSecond / 1_000_000_000).toFixed(1)} Gbps`;
  if (bitsPerSecond >= 1_000_000)
    return `${(bitsPerSecond / 1_000_000).toFixed(1)} Mbps`;
  if (bitsPerSecond >= 1_000)
    return `${(bitsPerSecond / 1_000).toFixed(1)} Kbps`;
  return `${Math.round(bitsPerSecond)} bps`;
};

const formatBytes = (bytes: number) => {
  if (bytes >= 1_000_000_000)
    return `${(bytes / 1_000_000_000).toFixed(2)} GB`;
  if (bytes >= 1_000_000)
    return `${(bytes / 1_000_000).toFixed(1)} MB`;
  if (bytes >= 1_000)
    return `${(bytes / 1_000).toFixed(1)} KB`;
  return `${Math.round(bytes)} B`;
};

function SummaryCard({
  icon: Icon,
  label,
  value,
  unit,
  progress,
  details,
}: {
  icon: LucideIcon;
  label: string;
  value: string | number;
  unit?: string;
  progress?: number;
  details: Array<{ label: string; value: string | number }>;
}) {
  return (
    <section className="overview-summary-card">
      <header>
        <span className="overview-summary-icon">
          <Icon size={17} strokeWidth={1.8} />
        </span>
        <h2>{label}</h2>
      </header>
      <div className="overview-summary-value">
        <strong>{value}</strong>
        {unit ? <span>{unit}</span> : null}
      </div>
      {typeof progress === 'number' ? (
        <div className="overview-summary-progress" aria-hidden="true">
          <span style={{ width: `${Math.min(100, Math.max(0, progress))}%` }} />
        </div>
      ) : null}
      <dl>
        {details.map((detail) => (
          <div key={detail.label}>
            <dt>{detail.label}</dt>
            <dd>{detail.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

function TrafficChart({
  data,
  emptyText,
  hours, setHours, now,
}: {
  data: TrafficPoint[];
  emptyText: string;
  hours: number;
  setHours: (hours: number) => void;
  now: number;
}) {
  const {tr}=useI18n();
  const [hidden,setHidden]=useState<string[]>([]);
  const chartRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(1000);
  const height = 285;
  const padding = { left: 64, right: 20, top: 24, bottom: 42 };
  useEffect(() => {
    const element = chartRef.current;
    if (!element) return;
    const updateWidth = () => setWidth(Math.max(240, Math.round(element.getBoundingClientRect().width)));
    updateWidth();
    const observer = new ResizeObserver(updateWidth);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  const minTime = now - hours * 60 * 60 * 1000;
  const maxTime = now;
  const visible=data.filter(item=>!hidden.includes(item.application)&&item.time.getTime()>=minTime&&item.time.getTime()<=now);
  const maxValue = Math.max(1, ...visible.map((item) => item.value));
  const applications = [...new Set(data.map((item) => item.application))];
  const x = (time: number) => padding.left + ((time - minTime) / Math.max(1, maxTime - minTime)) * (width - padding.left - padding.right);
  const y = (value: number) => height - padding.bottom - (value / maxValue) * (height - padding.top - padding.bottom);
  const yTicks = [0, 0.25, 0.5, 0.75, 1];
  const xTicks = width < 500 ? [0, 0.5, 1] : [0, 0.2, 0.4, 0.6, 0.8, 1];
  const seriesColor = (index: number) =>
    chartColors[index % chartColors.length];

  return (
    <div className="overview-chart" ref={chartRef}>
      <div className="overview-chart-toolbar">
      <div className="overview-chart-legend">
        {applications.map((application, index) => (
          <button key={application} aria-pressed={!hidden.includes(application)} onClick={()=>setHidden(v=>v.includes(application)?v.filter(a=>a!==application):[...v,application])}>
            <i style={{ background: seriesColor(index) }} />
            <span title={application}>{application}</span>
          </button>
        ))}
      </div>
      <div className="overview-chart-controls" aria-label={tr('时间范围','Time range')}>{[1,6,24].map(h=><button key={h} aria-pressed={hours===h} onClick={()=>setHours(h)}>{h}{tr(' 小时','h')}</button>)}</div>
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Application traffic chart">
        <line x1={padding.left} x2={width - padding.right} y1={height - padding.bottom} y2={height - padding.bottom} className="overview-chart-axis" />
        {yTicks.map((tick) => {
          const tickY = y(maxValue * tick);
          return <g key={tick}><line x1={padding.left} x2={width - padding.right} y1={tickY} y2={tickY} className="overview-chart-grid" /><text x={padding.left - 10} y={tickY + 4} textAnchor="end">{formatTraffic(maxValue * tick)}</text></g>;
        })}
        {xTicks.map((tick) => {
          const tickTime = minTime + (maxTime - minTime) * tick;
          const tickX = x(tickTime);
          return <g key={`x-${tick}`}><line x1={tickX} x2={tickX} y1={height - padding.bottom} y2={height - padding.bottom + 5} className="overview-chart-axis" /><text x={tickX} y={height - 15} textAnchor={tick === 0 ? 'start' : tick === 1 ? 'end' : 'middle'}>{formatChartTime(new Date(tickTime))}</text></g>;
        })}
        {applications.map((application, index) => {
          const applicationData = visible.filter((item) => item.application === application);
          const segments:TrafficPoint[][]=[];
          for(const point of applicationData){const last=segments.at(-1);if(!last||point.time.getTime()-last[last.length-1].time.getTime()>90*1000)segments.push([point]);else last.push(point);}
          const curve=(points:TrafficPoint[])=>points.map((p,i)=>{const px=x(p.time.getTime()),py=y(p.value);if(!i)return `M ${px},${py}`;const prev=points[i-1],ax=x(prev.time.getTime()),ay=y(prev.value),mid=(ax+px)/2;return `C ${mid},${ay} ${mid},${py} ${px},${py}`;}).join(' ');
          return (
            <g key={application}>
              {segments.map((segment,i)=><g key={i}><path
                d={curve(segment)}
                fill="none"
                stroke={seriesColor(index)}
                strokeWidth="1.8"
                strokeOpacity="0.82"
                strokeLinejoin="round"
                strokeLinecap="round"
              />
              {segment.length === 1 ? (
                <circle
                  cx={x(segment[0].time.getTime())}
                  cy={y(segment[0].value)}
                  r="4"
                  fill={seriesColor(index)}
                  stroke="rgb(var(--surface))"
                  strokeWidth="2"
                />
              ) : null}</g>)}
            </g>
          );
        })}
        {!visible.length ? <text className="overview-chart-empty-label" x={(padding.left + width - padding.right) / 2} y={(padding.top + height - padding.bottom) / 2} textAnchor="middle">{data.length?tr('所选范围暂无可见采样','No visible samples in this range'):emptyText}</text> : null}
      </svg>
    </div>
  );
}

const DashboardPage: React.FC = () => {
  const [searchParams] = useSearchParams();
  const { tr } = useI18n();
  const [loading, setLoading] = useState(true);
  const [deviceData, setDeviceData] = useState<DistributionItem[]>([]);
  const [applicationData, setApplicationData] = useState<DistributionItem[]>(
    [],
  );
  const [edgeData, setEdgeData] = useState<DistributionItem[]>([]);
  const [trafficData, setTrafficData] = useState<TrafficPoint[]>([]);
  const [trafficBytes, setTrafficBytes] = useState(0);
  const [trafficError, setTrafficError] = useState(false);
  const [hours,setHours]=useState(1);
  const [sampleEnd,setSampleEnd]=useState(Date.now());
  const trafficCache=useRef(new Map<number,{points:TrafficPoint[];bytes:number;end:number}>());

  const loadDashboard = useCallback(async (signal:AbortSignal) => {
    setLoading(true);
    try {
      const [devicesResponse, applicationsResponse, edgesResponse] =
        await Promise.all([
          getDeviceList({ page_size: 1000 }),
          getApplicationList({ page_size: 1000 }),
          getEdgeList({ page_size: 1000 }),
        ]);
      if(signal.aborted)return;
      if([devicesResponse,applicationsResponse,edgesResponse].some(r=>r.code!==200))throw Error('Resource query failed');
      const devices = devicesResponse.data?.devices || [];
      const applications = applicationsResponse.data?.applications || [];
      const edges = edgesResponse.data?.edges || [];

      const deviceCounts = new Map<string, number>();
      devices.forEach((device) => {
        const os = String(device.os || '').toLowerCase();
        const label = os.includes('linux')
          ? 'Linux'
          : os.includes('darwin') || os.includes('mac')
          ? 'macOS'
          : os.includes('windows')
          ? 'Windows'
          : tr('其他', 'Other');
        deviceCounts.set(label, (deviceCounts.get(label) || 0) + 1);
      });
      setDeviceData(
        [...deviceCounts].map(([type, value]) => ({ type, value })),
      );

      const applicationCounts = new Map<string, number>();
      applications.forEach((application) => {
        const type = String(application.application_type || '').toLowerCase();
        const label = applicationLabels[type] || type.toUpperCase() || '-';
        applicationCounts.set(label, (applicationCounts.get(label) || 0) + 1);
      });
      setApplicationData(
        [...applicationCounts].map(([type, value]) => ({ type, value })),
      );

      const online = edges.filter((edge) => edge.online === 1).length;
      setEdgeData(
        edges.length
          ? [
              { type: tr('在线', 'Online'), value: online },
              { type: tr('离线', 'Offline'), value: edges.length - online },
            ]
          : [],
      );

      const endTime = new Date();
      const startTime = new Date(endTime.getTime() - hours * 60 * 60 * 1000);
      // Split dense minute samples into non-overlapping windows so idle samples
      // do not consume the whole day's query limit and hide recent activity.
      const windows = [];
      for (let batch = 0; batch < hours; batch += 4) {
      if(signal.aborted)return;
      windows.push(...await Promise.all(Array.from({length: Math.min(4,hours-batch)}, (_, index) => {
        const hour = batch + index;
        const from = new Date(startTime.getTime() + hour * 3600000);
        const to = new Date(Math.min(endTime.getTime(), from.getTime() + 3600000));
        return getTrafficMetricsList({start_time: from.toISOString(), end_time: to.toISOString(), limit:10000},signal);
      })));
      }
      if(signal.aborted)return;
      if(windows.some(response=>response.code!==200))throw Error('Traffic query failed');
      if (windows.some(response => (response.data?.metrics?.length || 0) >= 10000)) {
        throw new Error('Traffic sample window reached its query limit');
      }
      const metrics = [...new Map(windows.flatMap(response => response.data?.metrics || []).map(metric =>
        [`${metric.application_id}:${metric.proxy_id}:${metric.timestamp}`, metric] as const)).values()];
      const applicationNames = new Map(
        applications.map((application) => [application.id, application.name]),
      );
      const buckets = new Map<
        string,
        { time: Date; application: string; bytes: number; samples: number }
      >();
      let totalBytes = 0;

      metrics.forEach((metric) => {
        const date = new Date(metric.timestamp);
        if (Number.isNaN(date.getTime()) || date.getTime()<startTime.getTime() || date.getTime()>endTime.getTime()) return;
        date.setSeconds(0, 0);
        const bytes =
          numberValue(metric.bytes_in) + numberValue(metric.bytes_out);
        totalBytes += bytes;
        const application =
          applicationNames.get(metric.application_id) ||
          `${tr('应用', 'Application')} #${metric.application_id}`;
        const key = `${date.getTime()}:${metric.application_id}`;
        const current = buckets.get(key) || {
          time: date,
          application,
          bytes: 0,
          samples: 0,
        };
        current.bytes += bytes;
        current.samples += 1;
        buckets.set(key, current);
      });

      setTrafficBytes(totalBytes);
      setTrafficError(false);
      const points = [...buckets.values()]
          .map((bucket) => ({
            time: bucket.time,
            application: bucket.application,
            value: (bucket.bytes * 8) / 60,
          }))
          .sort((left, right) => left.time.getTime() - right.time.getTime());
      setTrafficData(points);
      setSampleEnd(endTime.getTime());
      trafficCache.current.set(hours,{points,bytes:totalBytes,end:endTime.getTime()});
    } catch {
      if(signal.aborted)return;
      // Preserve the latest successful dashboard snapshot on transient failures.
      setTrafficError(true);
    } finally {
      if(!signal.aborted)setLoading(false);
    }
  }, [tr,hours]);

  useEffect(() => {
    const controller=new AbortController();let running=false;
    const cached=trafficCache.current.get(hours);
    setTrafficData(cached?.points||[]);setTrafficBytes(cached?.bytes||0);setSampleEnd(cached?.end||Date.now());setTrafficError(false);
    const refresh=async()=>{if(running)return;running=true;try{await loadDashboard(controller.signal);}finally{running=false;}};
    if(!cached||Date.now()-cached.end>=30000)void refresh();else setLoading(false);
    const timer = window.setInterval(() => void refresh(), 30_000);
    return () => {controller.abort();window.clearInterval(timer);};
  }, [loadDashboard]);

  const deviceTotal = useMemo(
    () => deviceData.reduce((total, item) => total + item.value, 0),
    [deviceData],
  );
  const applicationTotal = useMemo(
    () => applicationData.reduce((total, item) => total + item.value, 0),
    [applicationData],
  );
  const edgeTotal = useMemo(
    () => edgeData.reduce((total, item) => total + item.value, 0),
    [edgeData],
  );
  const onlineEdges = edgeData[0]?.value || 0;
  const totalTrafficData = useMemo(() => {
    const totals = new Map<number, number>();
    trafficData.forEach((point) => {
      const timestamp = point.time.getTime();
      totals.set(timestamp, (totals.get(timestamp) || 0) + point.value);
    });
    return [...totals]
      .map(([timestamp, value]) => ({
        time: new Date(timestamp),
        application: tr('总流量', 'Total traffic'),
        value,
      }))
      .sort((left, right) => left.time.getTime() - right.time.getTime());
  }, [trafficData, tr]);
  const otherTrafficLabel = tr('其他应用', 'Other applications');
  const trafficOverview = useMemo(() => {
    const applicationTotals = new Map<string, number>();
    trafficData.forEach((point) => {
      applicationTotals.set(
        point.application,
        (applicationTotals.get(point.application) || 0) + point.value,
      );
    });
    const rankedApplications = [...applicationTotals]
      .map(([application, value]) => ({ application, value }))
      .sort((left, right) => right.value - left.value);
    const topApplications = rankedApplications.slice(0, 4);
    const topApplicationNames = new Set(
      topApplications.map((item) => item.application),
    );
    const visibleBuckets = new Map<string, TrafficPoint>();

    trafficData.forEach((point) => {
      const application = topApplicationNames.has(point.application)
        ? point.application
        : otherTrafficLabel;
      const key = `${point.time.getTime()}:${application}`;
      const current = visibleBuckets.get(key);
      visibleBuckets.set(key, {
        time: point.time,
        application,
        value: (current?.value || 0) + point.value,
      });
    });

    const otherValue = rankedApplications
      .slice(4)
      .reduce((total, item) => total + item.value, 0);
    const visibleSeriesOrder = [
      ...topApplications.map((item) => item.application),
      ...(otherValue > 0 ? [otherTrafficLabel] : []),
    ];
    const visibleSeriesRank = new Map(
      visibleSeriesOrder.map((application, index) => [application, index]),
    );
    const composition = [
      ...topApplications,
      ...(otherValue > 0
        ? [{ application: otherTrafficLabel, value: otherValue }]
        : []),
    ];
    const compositionTotal = composition.reduce(
      (total, item) => total + item.value,
      0,
    );
    const latestPoint = totalTrafficData.at(-1);

    return {
      chartData: [...visibleBuckets.values()].sort((left, right) => {
        const seriesDifference =
          (visibleSeriesRank.get(left.application) || 0) -
          (visibleSeriesRank.get(right.application) || 0);
        return seriesDifference || left.time.getTime() - right.time.getTime();
      }),
      composition: composition.map((item) => ({
        ...item,
        share: compositionTotal ? (item.value / compositionTotal) * 100 : 0,
      })),
      latestRate: latestPoint?.value || 0,
      peakRate: Math.max(0, ...totalTrafficData.map((point) => point.value)),
      activeApplications: rankedApplications.filter(item => item.value > 0).length,
    };
  }, [otherTrafficLabel, totalTrafficData, trafficData]);

  if (searchParams.get('view') === 'agent') return <Navigate to="/" replace />;

  return (
    <div className="overview-page">
      <div className="overview-content" aria-busy={loading}>
        <div className="overview-summaries">
          <SummaryCard
            icon={HardDrive}
            label={tr('资源概览', 'Resource overview')}
            value={deviceTotal + applicationTotal}
            unit={tr('项资源', 'resources')}
            details={[
              { label: tr('设备', 'Devices'), value: deviceTotal },
              { label: tr('应用', 'Applications'), value: applicationTotal },
            ]}
          />
          <SummaryCard
            icon={Cable}
            label={tr('连接器可用性', 'Connector availability')}
            value={`${onlineEdges} / ${edgeTotal}`}
            progress={edgeTotal ? (onlineEdges / edgeTotal) * 100 : 0}
            details={[
              { label: tr('在线', 'Online'), value: onlineEdges },
              { label: tr('离线', 'Offline'), value: Math.max(0, edgeTotal - onlineEdges) },
            ]}
          />
          <SummaryCard
            icon={Activity}
            label={tr(`${hours} 小时流量`, `${hours}h traffic`)}
            value={loading&&!trafficCache.current.has(hours)?'—':formatBytes(trafficBytes)}
            details={[
              {
                label: tr('平均速率', 'Average rate'),
                value: formatTraffic((trafficBytes * 8) / (hours * 60 * 60)),
              },
              {
                label: tr('活跃应用', 'Active applications'),
                value: trafficOverview.activeApplications,
              },
            ]}
          />
        </div>

        <section className="overview-panel overview-traffic overview-traffic-unified">
          <header>
            <div>
              <h2>{tr('流量趋势', 'Traffic trend')}</h2>
              <p>{tr(`活跃应用流量，最近 ${hours} 小时`, `Active application traffic over the last ${hours} ${hours === 1 ? 'hour' : 'hours'}`)}</p>
              {loading&&<p role="status">{tr('正在更新…','Updating…')}</p>}
              {trafficError && <p role="status">{trafficCache.current.has(hours)?tr('流量数据未完整加载，当前保留上次结果。','Traffic data could not be fully loaded. Previous results are retained.'):tr('流量数据加载失败，请稍后重试。','Could not load traffic data. Please try again later.')}</p>}
            </div>
            <span>{tr('1 分钟粒度', '1-minute intervals')}</span>
          </header>
          <div className="overview-traffic-kpis">
            <div>
              <span>{tr('最近速率', 'Latest rate')}</span>
              <strong>{formatTraffic(trafficOverview.latestRate)}</strong>
            </div>
            <div>
              <span>{tr('峰值速率', 'Peak rate')}</span>
              <strong>{formatTraffic(trafficOverview.peakRate)}</strong>
            </div>
            <div>
              <span>{tr('活跃应用', 'Active applications')}</span>
              <strong>{trafficOverview.activeApplications}</strong>
            </div>
          </div>
          <div className="overview-traffic-layout">
            <TrafficChart
              data={trafficOverview.chartData}
              hours={hours} setHours={setHours} now={sampleEnd}
              emptyText={loading?tr('加载中…','Loading…'):tr('所选范围暂无流量', 'No traffic in this range')}
            />
            <aside className="overview-traffic-composition">
              <header>
                <h3>{tr('应用构成', 'Application mix')}</h3>
                <span>{tr('Top 4 + 其他', 'Top 4 + other')}</span>
              </header>
              <div className="overview-traffic-composition-list">
                {trafficOverview.composition.length ? (
                  trafficOverview.composition.map((item, index) => (
                    <div key={item.application}>
                      <span className="overview-traffic-composition-name">
                        <i style={{ background: chartColors[index % chartColors.length] }} />
                        <b title={item.application}>{item.application}</b>
                      </span>
                      <strong>{item.share.toFixed(1)}%</strong>
                      <span className="overview-traffic-composition-bar">
                        <i
                          style={{
                            width: `${item.share}%`,
                            background: chartColors[index % chartColors.length],
                          }}
                        />
                      </span>
                    </div>
                  ))
                ) : (
                  <p>{tr('暂无应用流量', 'No application traffic')}</p>
                )}
              </div>
            </aside>
          </div>
        </section>
      </div>
    </div>
  );
};

export default DashboardPage;
