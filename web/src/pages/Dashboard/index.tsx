import { APPLICATION_TYPES } from '@/constants/applicationTypes';
import { useI18n } from '@/i18n';
import {
  getApplicationList,
  getDeviceList,
  getEdgeList,
  getTrafficMetricsList,
} from '@/services/api';
import { useThemeMode } from '@/store/theme';
import { Line } from '@ant-design/plots';
import { PageContainer } from '@ant-design/pro-components';
import {
  Activity,
  AppWindow,
  Cable,
  HardDrive,
  type LucideIcon,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import './index.less';

type DistributionItem = { type: string; value: number };
type TrafficPoint = { time: Date; application: string; value: number };

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

const formatTraffic = (bitsPerSecond: number) => {
  if (bitsPerSecond >= 1_000_000_000)
    return `${(bitsPerSecond / 1_000_000_000).toFixed(1)} Gbps`;
  if (bitsPerSecond >= 1_000_000)
    return `${(bitsPerSecond / 1_000_000).toFixed(1)} Mbps`;
  if (bitsPerSecond >= 1_000)
    return `${(bitsPerSecond / 1_000).toFixed(1)} Kbps`;
  return `${Math.round(bitsPerSecond)} bps`;
};

function MetricCard({
  icon: Icon,
  label,
  value,
  hint,
}: {
  icon: LucideIcon;
  label: string;
  value: string | number;
  hint: string;
}) {
  return (
    <div className="overview-metric">
      <div className="overview-metric-icon">
        <Icon size={16} strokeWidth={1.8} />
      </div>
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{hint}</small>
    </div>
  );
}

function DistributionPanel({
  title,
  hint,
  items,
  emptyText,
}: {
  title: string;
  hint: string;
  items: DistributionItem[];
  emptyText: string;
}) {
  const max = Math.max(1, ...items.map((item) => item.value));

  return (
    <section className="overview-panel overview-distribution">
      <header>
        <div>
          <h2>{title}</h2>
          <p>{hint}</p>
        </div>
        <span>{items.reduce((total, item) => total + item.value, 0)}</span>
      </header>
      <div className="overview-distribution-list">
        {items.length > 0 ? (
          items.map((item) => (
            <div className="overview-distribution-row" key={item.type}>
              <div className="overview-distribution-meta">
                <span>{item.type}</span>
                <strong>{item.value}</strong>
              </div>
              <div className="overview-distribution-track">
                <span style={{ width: `${(item.value / max) * 100}%` }} />
              </div>
            </div>
          ))
        ) : (
          <div className="overview-empty">{emptyText}</div>
        )}
      </div>
    </section>
  );
}

const DashboardPage: React.FC = () => {
  const { tr } = useI18n();
  const { resolved } = useThemeMode();
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
  const axisColor = resolved === 'dark' ? '#71717a' : '#64748b';
  const gridColor = resolved === 'dark' ? '#292933' : '#e2e8f0';

  return (
    <PageContainer title={false} className="overview-page">
      <div className={`overview-content${loading ? ' is-loading' : ''}`}>
        <div className="overview-metrics">
          <MetricCard
            icon={HardDrive}
            label={tr('设备', 'Devices')}
            value={deviceTotal}
            hint={tr('已纳管终端', 'Managed endpoints')}
          />
          <MetricCard
            icon={AppWindow}
            label={tr('应用', 'Applications')}
            value={applicationTotal}
            hint={tr('私有服务', 'Private services')}
          />
          <MetricCard
            icon={Cable}
            label={tr('连接器', 'Connectors')}
            value={`${onlineEdges} / ${edgeTotal}`}
            hint={tr('在线 / 总数', 'Online / total')}
          />
          <MetricCard
            icon={Activity}
            label={tr('24 小时流量', '24h traffic')}
            value={formatTraffic((trafficBytes * 8) / (24 * 60 * 60))}
            hint={tr('平均吞吐', 'Average throughput')}
          />
        </div>

        <div className="overview-distributions">
          <DistributionPanel
            title={tr('设备系统', 'Device OS')}
            hint={tr('按操作系统分布', 'Distribution by operating system')}
            items={deviceData}
            emptyText={tr('暂无设备', 'No devices')}
          />
          <DistributionPanel
            title={tr('应用类型', 'Application types')}
            hint={tr('按协议分布', 'Distribution by protocol')}
            items={applicationData}
            emptyText={tr('暂无应用', 'No applications')}
          />
          <DistributionPanel
            title={tr('连接器状态', 'Connector status')}
            hint={tr('当前在线状态', 'Current availability')}
            items={edgeData}
            emptyText={tr('暂无连接器', 'No connectors')}
          />
        </div>

        <section className="overview-panel overview-traffic">
          <header>
            <div>
              <h2>{tr('应用流量', 'Application traffic')}</h2>
              <p>
                {tr(
                  '最近 24 小时，10 分钟粒度',
                  'Last 24 hours, 10-minute intervals',
                )}
              </p>
            </div>
          </header>
          {trafficData.length > 0 ? (
            <Line
              data={trafficData}
              xField="time"
              yField="value"
              colorField="application"
              height={260}
              point={false}
              smooth
              style={{ lineWidth: 1.6, opacity: 0.9 }}
              legend={{ position: 'top-right' }}
              scale={{ value: { min: 0, nice: true } }}
              axis={{
                x: {
                  labelAutoHide: 'greedy',
                  labelFill: axisColor,
                  line: false,
                  tick: false,
                },
                y: {
                  labelFill: axisColor,
                  labelFormatter: (value: number) => formatTraffic(value),
                  gridStroke: gridColor,
                  line: false,
                  tick: false,
                },
              }}
              tooltip={{
                shared: true,
                valueFormatter: (value: number) => formatTraffic(value),
              }}
            />
          ) : (
            <div className="overview-traffic-empty">
              <Activity size={18} strokeWidth={1.7} />
              <span>
                {tr('最近 24 小时暂无流量', 'No traffic in the last 24 hours')}
              </span>
            </div>
          )}
        </section>
      </div>
    </PageContainer>
  );
};

export default DashboardPage;
