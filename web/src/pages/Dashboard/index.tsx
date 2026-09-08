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

const formatLocalTime = (date: Date) => {
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate(),
  )}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(
    date.getSeconds(),
  )}`;
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
}: {
  data: TrafficPoint[];
  emptyText: string;
}) {
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
  const now = Date.now();
  const minTime = now - 24 * 60 * 60 * 1000;
  const maxTime = now;
  const maxValue = Math.max(1, ...data.map((item) => item.value));
  const applications = [...new Set(data.map((item) => item.application))];
  const x = (time: number) => padding.left + ((time - minTime) / Math.max(1, maxTime - minTime)) * (width - padding.left - padding.right);
  const y = (value: number) => height - padding.bottom - (value / maxValue) * (height - padding.top - padding.bottom);
  const yTicks = [0, 0.25, 0.5, 0.75, 1];
  const xTicks = [0, 0.2, 0.4, 0.6, 0.8, 1];
  const seriesColor = (index: number) =>
    chartColors[index % chartColors.length];

  return (
    <div className="overview-chart" ref={chartRef}>
      <div className="overview-chart-legend">
        {applications.map((application, index) => (
          <span key={application}>
            <i style={{ background: seriesColor(index) }} />
            {application}
          </span>
        ))}
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
          const applicationData = data.filter((item) => item.application === application);
          const points = applicationData.map((item) => `${x(item.time.getTime())},${y(item.value)}`).join(' ');
          return (
            <g key={application}>
              <polyline
                points={points}
                fill="none"
                stroke={seriesColor(index)}
                strokeWidth="1.8"
                strokeOpacity="0.82"
                strokeLinejoin="round"
                strokeLinecap="round"
              />
              {applicationData.length === 1 ? (
                <circle
                  cx={x(applicationData[0].time.getTime())}
                  cy={y(applicationData[0].value)}
                  r="4"
                  fill={seriesColor(index)}
                  stroke="rgb(var(--surface))"
                  strokeWidth="2"
                />
              ) : null}
            </g>
          );
        })}
        {!data.length ? <text className="overview-chart-empty-label" x={(padding.left + width - padding.right) / 2} y={(padding.top + height - padding.bottom) / 2} textAnchor="middle">{emptyText}</text> : null}
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

  const loadDashboard = useCallback(async () => {
    setLoading(true);
    try {
      const [devicesResponse, applicationsResponse, edgesResponse] =
        await Promise.all([
          getDeviceList({ page_size: 1000 }),
          getApplicationList({ page_size: 1000 }),
          getEdgeList({ page_size: 1000 }),
        ]);
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
      const startTime = new Date(endTime.getTime() - 24 * 60 * 60 * 1000);
      const trafficResponse = await getTrafficMetricsList({
        start_time: formatLocalTime(startTime),
        end_time: formatLocalTime(endTime),
        limit: 10000,
      });
      const metrics = trafficResponse.data?.metrics || [];
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
        if (Number.isNaN(date.getTime())) return;
        date.setMinutes(Math.floor(date.getMinutes() / 10) * 10, 0, 0);
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
      setTrafficData(
        [...buckets.values()]
          .map((bucket) => ({
            time: bucket.time,
            application: bucket.application,
            value: ((bucket.bytes / Math.max(1, bucket.samples)) * 8) / 60,
          }))
          .sort((left, right) => left.time.getTime() - right.time.getTime()),
      );
    } catch {
      // Preserve the latest successful dashboard snapshot on transient failures.
    } finally {
      setLoading(false);
    }
  }, [tr]);

  useEffect(() => {
    void loadDashboard();
    const timer = window.setInterval(() => void loadDashboard(), 30_000);
    return () => window.clearInterval(timer);
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
      activeApplications: rankedApplications.length,
    };
  }, [otherTrafficLabel, totalTrafficData, trafficData]);

  if (searchParams.get('view') === 'agent') return <Navigate to="/" replace />;

  return (
    <div className="overview-page">
      <div className={`overview-content${loading ? ' is-loading' : ''}`}>
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
            label={tr('24 小时流量', '24h traffic')}
            value={formatBytes(trafficBytes)}
            details={[
              {
                label: tr('平均速率', 'Average rate'),
                value: formatTraffic((trafficBytes * 8) / (24 * 60 * 60)),
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
              <p>{tr('活跃应用流量，最近 24 小时', 'Active application traffic over the last 24 hours')}</p>
            </div>
            <span>{tr('10 分钟粒度', '10-minute intervals')}</span>
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
              emptyText={tr('最近 24 小时暂无流量', 'No traffic in the last 24 hours')}
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
