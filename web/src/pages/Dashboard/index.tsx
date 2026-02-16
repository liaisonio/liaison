import { PageContainer } from '@ant-design/pro-components';
import { Card, Row, Col, Spin } from 'antd';
import { Pie, Line } from '@ant-design/plots';
import { useEffect, useState } from 'react';
import { BarChartOutlined, PieChartOutlined, LineChartOutlined } from '@ant-design/icons';
import { getDeviceList, getApplicationList, getEdgeList, getTrafficMetricsList } from '@/services/api';

interface PieData {
  type: string;
  value: number;
}

interface ApplicationTrafficData {
  application: string; // 应用名称
  application_id: number;
  bytes_in: number;
  bytes_out: number;
}

interface TimeTrafficData {
  time: string; // 时间戳
  application: string; // 应用名称
  bytes_in: number;
  bytes_out: number;
}

const DashboardPage: React.FC = () => {
  const [loading, setLoading] = useState(true);
  const [deviceData, setDeviceData] = useState<PieData[]>([]);
  const [applicationData, setApplicationData] = useState<PieData[]>([]);
  const [edgeData, setEdgeData] = useState<PieData[]>([]);
  const [timeTrafficData, setTimeTrafficData] = useState<TimeTrafficData[]>([]);

  useEffect(() => {
    loadData();
    // 每30秒刷新一次流量数据
    const interval = setInterval(() => {
      loadTrafficData();
    }, 30000);
    return () => clearInterval(interval);
  }, []);

  const loadData = async () => {
    setLoading(true);
    try {
      // 获取所有数据
      const [devicesRes, applicationsRes, edgesRes] = await Promise.all([
        getDeviceList({ page_size: 1000 }),
        getApplicationList({ page_size: 1000 }),
        getEdgeList({ page_size: 1000 }),
      ]);

      // 处理设备数据 - 按操作系统分类
      const devices = devicesRes.data?.devices || [];
      const deviceStats: Record<string, number> = {};
      devices.forEach((device: API.Device) => {
        const os = device.os?.toLowerCase() || 'unknown';
        let osType = '其他';
        if (os.includes('linux')) {
          osType = 'Linux';
        } else if (os.includes('darwin') || os.includes('mac')) {
          osType = 'macOS';
        } else if (os.includes('windows')) {
          osType = 'Windows';
        }
        deviceStats[osType] = (deviceStats[osType] || 0) + 1;
      });
      const deviceDataList = Object.entries(deviceStats).map(([type, value]) => ({ type, value }));
      setDeviceData(deviceDataList);

      // 处理应用数据 - 按应用类型分类
      const applications = applicationsRes.data?.applications || [];
      const appStats: Record<string, number> = {};
      const typeMap: Record<string, string> = {
        http: 'HTTP',
        tcp: 'TCP',
        udp: 'UDP',
        ssh: 'SSH',
        rdp: 'RDP',
        database: '数据库',
        mysql: 'MySQL',
        postgresql: 'PostgreSQL',
        redis: 'Redis',
        mongodb: 'MongoDB',
      };
      applications.forEach((app: API.Application) => {
        const type = app.application_type || 'unknown';
        const displayType = typeMap[type.toLowerCase()] || type.toUpperCase();
        appStats[displayType] = (appStats[displayType] || 0) + 1;
      });
      const applicationDataList = Object.entries(appStats).map(([type, value]) => ({ type, value }));
      setApplicationData(applicationDataList);

      // 处理连接器数据 - 按在线状态分类
      const edges = edgesRes.data?.edges || [];
      const edgeStats: Record<string, number> = {
        在线: 0,
        离线: 0,
      };
      edges.forEach((edge: API.Edge) => {
        if (edge.online === 1) {
          edgeStats['在线']++;
        } else {
          edgeStats['离线']++;
        }
      });
      // 只有当有连接器数据时才显示，否则显示空状态
      const totalEdges = edgeStats['在线'] + edgeStats['离线'];
      const edgeDataList = totalEdges > 0 
        ? Object.entries(edgeStats).map(([type, value]) => ({ type, value }))
        : [];
      setEdgeData(edgeDataList);

      // 调试信息
      console.log('设备数据:', deviceDataList);
      console.log('应用数据:', applicationDataList);
      console.log('连接器数据:', edgeDataList);
    } catch (error) {
      console.error('加载数据失败:', error);
    } finally {
      setLoading(false);
    }
    // 加载流量数据
    loadTrafficData();
  };

  const loadTrafficData = async () => {
    try {
      // 获取应用列表，建立应用ID到名称的映射
      const applicationsRes = await getApplicationList({ page_size: 1000 });
      const applications = applicationsRes.data?.applications || [];
      const appMap: Record<number, string> = {};
      applications.forEach((app: API.Application) => {
        appMap[app.id] = app.name;
      });

      // 获取最近24小时的流量数据
      const endTime = new Date();
      const startTime = new Date(endTime.getTime() - 24 * 60 * 60 * 1000); // 24小时前
      
      // 格式化为本地时间字符串（不带时区）：YYYY-MM-DDTHH:mm:ss
      const formatLocalTime = (date: Date): string => {
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        const hours = String(date.getHours()).padStart(2, '0');
        const minutes = String(date.getMinutes()).padStart(2, '0');
        const seconds = String(date.getSeconds()).padStart(2, '0');
        return `${year}-${month}-${day}T${hours}:${minutes}:${seconds}`;
      };
      
      const res = await getTrafficMetricsList({
        start_time: formatLocalTime(startTime),
        end_time: formatLocalTime(endTime),
        limit: 10000, // 增加限制以获取更多数据
      });

      const metrics = res.data?.metrics || [];
      console.log('获取到的流量数据:', metrics.length, '条');
      if (metrics.length > 0) {
        console.log('第一条数据示例:', metrics[0]);
      }
      
      // 生成完整的时间序列（最近24小时，每10分钟一个点）
      const sortedTimes: string[] = [];
      const now = new Date();
      // 对齐到最近的10分钟
      const alignedMinutes = Math.floor(now.getMinutes() / 10) * 10;
      const alignedNow = new Date(now);
      alignedNow.setMinutes(alignedMinutes, 0, 0);
      
      // 从当前时间往前推24小时，生成144个10分钟间隔的数据点
      for (let i = 143; i >= 0; i--) {
        const time = new Date(alignedNow.getTime() - i * 10 * 60 * 1000);
        // 存储完整时间（包含分钟，用于数据聚合）
        const timeKey = `${String(time.getMonth() + 1).padStart(2, '0')}-${String(time.getDate()).padStart(2, '0')} ${String(time.getHours()).padStart(2, '0')}:${String(time.getMinutes()).padStart(2, '0')}`;
        sortedTimes.push(timeKey);
      }
      
      // 按时间和应用分组流量数据
      const timeAppMap: Record<string, Record<number, { bytes_in: number; bytes_out: number }>> = {};
      
      // 初始化所有时间点的数据结构
      sortedTimes.forEach((timeKey) => {
        timeAppMap[timeKey] = {};
        applications.forEach((app: API.Application) => {
          timeAppMap[timeKey][app.id] = { bytes_in: 0, bytes_out: 0 };
        });
      });
      
      // 填充实际流量数据（将数据聚合到10分钟间隔，并计算平均值）
      const timeAppCount: Record<string, Record<number, number>> = {}; // 记录每个时间点每个应用的数据条数
      
      metrics.forEach((metric: API.TrafficMetric) => {
        // 解析时间戳（本地时间格式：YYYY-MM-DDTHH:mm:ss）
        // 如果时间戳不包含时区信息，将其视为本地时间
        let date: Date;
        if (metric.timestamp.includes('+') || metric.timestamp.includes('Z') || metric.timestamp.includes('T') && metric.timestamp.length > 19) {
          // 包含时区信息，使用标准解析
          date = new Date(metric.timestamp);
        } else {
          // 本地时间格式，手动解析为本地时间
          const [datePart, timePart] = metric.timestamp.split('T');
          const [year, month, day] = datePart.split('-').map(Number);
          const [hour, minute, second = 0] = (timePart || '').split(':').map(Number);
          date = new Date(year, month - 1, day, hour, minute, second);
        }
        // 对齐到10分钟间隔
        const alignedMinutes = Math.floor(date.getMinutes() / 10) * 10;
        const alignedDate = new Date(date);
        alignedDate.setMinutes(alignedMinutes, 0, 0);
        // 存储完整时间（包含分钟）
        const timeKey = `${String(alignedDate.getMonth() + 1).padStart(2, '0')}-${String(alignedDate.getDate()).padStart(2, '0')} ${String(alignedDate.getHours()).padStart(2, '0')}:${String(alignedDate.getMinutes()).padStart(2, '0')}`;
        
        if (timeAppMap[timeKey]) {
          const appId = metric.application_id;
          // 确保 timeAppCount[timeKey] 已初始化
          if (!timeAppCount[timeKey]) {
            timeAppCount[timeKey] = {};
          }
          if (!timeAppMap[timeKey][appId]) {
            timeAppMap[timeKey][appId] = { bytes_in: 0, bytes_out: 0 };
            timeAppCount[timeKey][appId] = 0;
          }
          // 确保转换为数字类型，累加流量
          const bytesIn = typeof metric.bytes_in === 'string' ? parseInt(metric.bytes_in, 10) : metric.bytes_in;
          const bytesOut = typeof metric.bytes_out === 'string' ? parseInt(metric.bytes_out, 10) : metric.bytes_out;
          timeAppMap[timeKey][appId].bytes_in += Number.isNaN(bytesIn) ? 0 : bytesIn;
          timeAppMap[timeKey][appId].bytes_out += Number.isNaN(bytesOut) ? 0 : bytesOut;
          // 记录数据条数
          timeAppCount[timeKey][appId] = (timeAppCount[timeKey][appId] || 0) + 1;
        }
      });
      
      // 计算平均值（每个10分钟间隔内的平均每分钟流量）
      Object.keys(timeAppMap).forEach((timeKey) => {
        Object.keys(timeAppMap[timeKey]).forEach((appIdStr) => {
          const appId = parseInt(appIdStr);
          const count = timeAppCount[timeKey]?.[appId] || 1; // 至少为1，避免除0
          // 计算平均值：总流量 / 数据条数（即平均每分钟流量）
          const bytesIn = timeAppMap[timeKey][appId].bytes_in;
          const bytesOut = timeAppMap[timeKey][appId].bytes_out;
          timeAppMap[timeKey][appId].bytes_in = count > 0 ? Math.round(bytesIn / count) : 0;
          timeAppMap[timeKey][appId].bytes_out = count > 0 ? Math.round(bytesOut / count) : 0;
        });
      });

      // 转换为图表数据格式：按时间排序，每个时间点包含所有应用的流量
      const timeTrafficDataList: TimeTrafficData[] = [];
      
      sortedTimes.forEach((time) => {
        const appData = timeAppMap[time] || {};
        // 确保所有应用在每个时间点都有数据（即使为0）
        applications.forEach((app: API.Application) => {
          const appId = app.id;
          const traffic = appData[appId] || { bytes_in: 0, bytes_out: 0 };
          // 将时间字符串转换为 Date 对象，用于图表库识别时间类型
          try {
            const timeParts = time.split(' ');
            if (timeParts.length < 2) {
              console.warn('时间格式错误:', time);
              return; // 跳过当前迭代
            }
            const datePart = timeParts[0];
            const timePart = timeParts[1];
            const dateParts = datePart.split('-');
            const timeParts2 = timePart.split(':');
            if (dateParts.length < 2 || timeParts2.length < 2) {
              console.warn('时间格式错误:', time, 'dateParts:', dateParts, 'timeParts2:', timeParts2);
              return; // 跳过当前迭代
            }
            const month = dateParts[0];
            const day = dateParts[1];
            const hour = timeParts2[0];
            const minute = timeParts2[1];
            // 使用当前年份，创建 Date 对象
            const currentYear = new Date().getFullYear();
            const dateObj = new Date(currentYear, parseInt(month) - 1, parseInt(day), parseInt(hour), parseInt(minute), 0);
            
            // 确保值为数字类型
            const bytesIn = typeof traffic.bytes_in === 'number' ? traffic.bytes_in : parseInt(String(traffic.bytes_in || 0), 10);
            const bytesOut = typeof traffic.bytes_out === 'number' ? traffic.bytes_out : parseInt(String(traffic.bytes_out || 0), 10);
            
            // 格式化为本地时间格式（YYYY-MM-DDTHH:mm:ss）
            const formatLocalTime = (d: Date): string => {
              const year = d.getFullYear();
              const month = String(d.getMonth() + 1).padStart(2, '0');
              const day = String(d.getDate()).padStart(2, '0');
              const hours = String(d.getHours()).padStart(2, '0');
              const minutes = String(d.getMinutes()).padStart(2, '0');
              const seconds = String(d.getSeconds()).padStart(2, '0');
              return `${year}-${month}-${day}T${hours}:${minutes}:${seconds}`;
            };
            
            timeTrafficDataList.push({
              time: formatLocalTime(dateObj), // 存储本地时间格式
              application: app.name,
              bytes_in: Number.isNaN(bytesIn) ? 0 : bytesIn,
              bytes_out: Number.isNaN(bytesOut) ? 0 : bytesOut,
            });
          } catch (error) {
            console.error('处理时间数据时出错:', time, error);
            // 跳过当前迭代，继续处理下一个
          }
        });
      });

      setTimeTrafficData(timeTrafficDataList);
    } catch (error) {
      console.error('加载流量数据失败:', error);
    }
  };

  const getPieConfig = (data: PieData[]) => {
    return {
      data,
      angleField: 'value',
      colorField: 'type',
      // 标签显示在外部，显示类型名称和数值
      label: {
        text: (d: any) => `${d.type}: ${d.value}`,
        position: 'outside',
        style: {
          fontSize: 12,
          fill: '#666',
        },
      },
      legend: false, // 隐藏默认图例，我们手动添加
      // @ant-design/plots 默认有 tooltip，显示类型和数值
      // 使用更丰富的颜色方案
      color: ['#1890ff', '#52c41a', '#faad14', '#f5222d', '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16'],
      height: 200,
      tooltip: false,
    };
  };

  return (
    <PageContainer>
      <Spin spinning={loading}>
        <Row gutter={[16, 16]}>
          <Col xs={24} sm={24} md={8}>
            <Card title="设备统计" variant="outlined">
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                {deviceData.length > 0 ? (
                  <>
                    <div style={{ flex: 1, minHeight: 200 }}>
                      <Pie {...getPieConfig(deviceData)} />
                    </div>
                    <div style={{ marginTop: 16, textAlign: 'center' }}>
                      {deviceData.map((item, index) => {
                        const colors = ['#1890ff', '#52c41a', '#faad14', '#f5222d', '#722ed1', '#13c2c2'];
                        const color = colors[index % colors.length];
                        return (
                          <span key={item.type} style={{ margin: '0 8px', fontSize: '12px' }}>
                            <span
                              style={{
                                display: 'inline-block',
                                width: 12,
                                height: 12,
                                backgroundColor: color,
                                marginRight: 4,
                                verticalAlign: 'middle',
                              }}
                            />
                            {item.type}: {item.value}
                          </span>
                        );
                      })}
                    </div>
                  </>
                ) : (
                  <div style={{ 
                    textAlign: 'center', 
                    padding: '60px 20px',
                    color: '#999',
                    fontSize: '14px'
                  }}>
                  <PieChartOutlined style={{ 
                    fontSize: '48px', 
                    marginBottom: '16px',
                    color: '#d9d9d9'
                  }} />
                  <div>暂无数据</div>
                  </div>
                )}
              </div>
            </Card>
          </Col>
          <Col xs={24} sm={24} md={8}>
            <Card title="应用统计" variant="outlined">
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                {applicationData.length > 0 ? (
                  <>
                    <div style={{ flex: 1, minHeight: 200 }}>
                      <Pie {...getPieConfig(applicationData)} />
                    </div>
                    <div style={{ marginTop: 16, textAlign: 'center' }}>
                      {applicationData.map((item, index) => {
                        const colors = ['#1890ff', '#52c41a', '#faad14', '#f5222d', '#722ed1', '#13c2c2'];
                        const color = colors[index % colors.length];
                        return (
                          <span key={item.type} style={{ margin: '0 8px', fontSize: '12px' }}>
                            <span
                              style={{
                                display: 'inline-block',
                                width: 12,
                                height: 12,
                                backgroundColor: color,
                                marginRight: 4,
                                verticalAlign: 'middle',
                              }}
                            />
                            {item.type}: {item.value}
                          </span>
                        );
                      })}
                    </div>
                  </>
                ) : (
                  <div style={{ 
                    textAlign: 'center', 
                    padding: '60px 20px',
                    color: '#999',
                    fontSize: '14px'
                  }}>
                  <PieChartOutlined style={{ 
                    fontSize: '48px', 
                    marginBottom: '16px',
                    color: '#d9d9d9'
                  }} />
                  <div>暂无数据</div>
                  </div>
                )}
              </div>
            </Card>
          </Col>
          <Col xs={24} sm={24} md={8}>
            <Card title="连接器统计" variant="outlined">
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                {edgeData.length > 0 ? (
                  <>
                    <div style={{ flex: 1, minHeight: 200 }}>
                      <Pie {...getPieConfig(edgeData)} />
                    </div>
                    <div style={{ marginTop: 16, textAlign: 'center' }}>
                      {edgeData.map((item, index) => {
                        // 连接器统计使用特定颜色：在线=蓝色，离线=灰色
                        const colors = item.type === '在线' ? '#1890ff' : '#d9d9d9';
                        return (
                          <span key={item.type} style={{ margin: '0 8px', fontSize: '12px' }}>
                            <span
                              style={{
                                display: 'inline-block',
                                width: 12,
                                height: 12,
                                backgroundColor: colors,
                                borderRadius: '2px',
                                marginRight: 4,
                                verticalAlign: 'middle',
                              }}
                            />
                            {item.type}: {item.value}
                          </span>
                        );
                      })}
                    </div>
                  </>
                ) : (
                  <div style={{ 
                    textAlign: 'center', 
                    padding: '60px 20px',
                    color: '#999',
                    fontSize: '14px'
                  }}>
                  <PieChartOutlined style={{ 
                    fontSize: '48px', 
                    marginBottom: '16px',
                    color: '#d9d9d9'
                  }} />
                  <div>暂无数据</div>
                  </div>
                )}
              </div>
            </Card>
          </Col>
        </Row>
        {/* 第二排：应用流量监控图表 */}
        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          <Col xs={24}>
            <Card title="应用流量监控" variant="outlined">
              {timeTrafficData.length > 0 ? (() => {
                // 计算所有数据的bps值，用于确定Y轴范围
                const dataWithBps = timeTrafficData.map((d) => {
                  const bytesIn = typeof d.bytes_in === 'number' ? d.bytes_in : parseInt(String(d.bytes_in || 0), 10);
                  const bytesOut = typeof d.bytes_out === 'number' ? d.bytes_out : parseInt(String(d.bytes_out || 0), 10);
                  const totalBytes = (Number.isNaN(bytesIn) ? 0 : bytesIn) + (Number.isNaN(bytesOut) ? 0 : bytesOut);
                  const bps = (totalBytes * 8) / 60;
                  return {
                    date: d.time,
                    type: d.application,
                    value: bps,
                  };
                });
                
                // 计算最大值，如果所有值都是0，设置一个小的非零值避免显示多个0刻度
                const maxValue = Math.max(...dataWithBps.map(d => d.value));
                const allZero = maxValue === 0;
                
                return (
                  <Line
                    data={dataWithBps}
                  xField={(d: any) => {
                    // 解析本地时间格式（YYYY-MM-DDTHH:mm:ss）
                    const dateStr = d.date;
                    if (dateStr.includes('+') || dateStr.includes('Z') || (dateStr.includes('T') && dateStr.length > 19)) {
                      // 包含时区信息，使用标准解析
                      return new Date(dateStr);
                    } else {
                      // 本地时间格式，手动解析为本地时间
                      const [datePart, timePart] = dateStr.split('T');
                      const [year, month, day] = datePart.split('-').map(Number);
                      const [hour, minute, second = 0] = (timePart || '').split(':').map(Number);
                      return new Date(year, month - 1, day, hour, minute, second);
                    }
                  }}
                  yField="value"
                  colorField="type"
                  height={350}
                  point={false}
                  smooth={true}
                  legend={{
                    position: 'top-right',
                    itemHeight: 14,
                    maxWidth: 300,
                  }}
                  scale={{
                    value: allZero ? {
                      domain: [0, 1],
                      nice: false,
                      ticks: [0, 1],
                    } : {
                      nice: true,
                      min: 0,
                    },
                  }}
                  axis={{
                    x: {
                      labelAutoHide: 'greedy',
                      labelTransform: 'rotate(-45)',
                      labelFill: '#999',
                      lineStroke: '#e8e8e8',
                      tickStroke: '#e8e8e8',
                    },
                    y: {
                      labelFill: '#666',
                      labelFormatter: (datum: any) => {
                        // 确保值是数字类型（单位：bps）
                        const value = typeof datum === 'number' ? datum : parseFloat(String(datum || 0));
                        if (isNaN(value) || value === 0) return '0 bps';
                        // 转换为合适的单位：bps, Kbps, Mbps, Gbps
                        if (value >= 1000 * 1000 * 1000) {
                          return `${(value / (1000 * 1000 * 1000)).toFixed(2)} Gbps`;
                        } else if (value >= 1000 * 1000) {
                          return `${(value / (1000 * 1000)).toFixed(2)} Mbps`;
                        } else if (value >= 1000) {
                          return `${(value / 1000).toFixed(2)} Kbps`;
                        }
                        return `${Math.round(value)} bps`;
                      },
                      lineStroke: '#e8e8e8',
                      tickStroke: '#e8e8e8',
                      tickCount: allZero ? 2 : 5, // 当所有值都是0时，只显示2个刻度（0和1）
                    },
                  }}
                  label={false}
                  tooltip={{
                    showCrosshairs: true,
                    shared: true,
                  }}
                />
                );
              })(              ) : (
                <div style={{ 
                  textAlign: 'center', 
                  padding: '60px 20px',
                  color: '#999',
                  fontSize: '14px'
                }}>
                  <LineChartOutlined style={{ 
                    fontSize: '48px', 
                    marginBottom: '16px',
                    color: '#d9d9d9'
                  }} />
                  <div>暂无流量数据</div>
                </div>
              )}
            </Card>
          </Col>
        </Row>
      </Spin>
    </PageContainer>
  );
};

export default DashboardPage;
