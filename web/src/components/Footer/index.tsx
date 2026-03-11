import { GithubOutlined } from '@ant-design/icons';
import { Typography, Divider } from 'antd';
import { APP_NAME } from '@/constants';

const { Text, Link } = Typography;
const GITHUB_URL = 'https://github.com/singchia/liaison';

const Footer: React.FC = () => {
  return (
    <div style={{ 
      marginTop: 48, 
      padding: '24px 0',
      borderTop: '1px solid rgba(0, 0, 0, 0.06)'
    }}>
      <div style={{ 
        display: 'flex', 
        flexDirection: 'column', 
        gap: 12,
        maxWidth: 1200,
        margin: '0 auto',
        padding: '0 24px'
      }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16, flexWrap: 'wrap' }}>
            <span style={{ fontWeight: 500, minWidth: 'fit-content', whiteSpace: 'nowrap' }}>产品名称:</span>
            <span>{APP_NAME}</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16, flexWrap: 'wrap' }}>
            <span style={{ fontWeight: 500, minWidth: 'fit-content', whiteSpace: 'nowrap' }}>GitHub:</span>
            <Link 
              href={GITHUB_URL} 
              target="_blank" 
              rel="noopener noreferrer"
              style={{ 
                display: 'inline-flex',
                alignItems: 'center',
                wordBreak: 'break-all',
                flex: 1
              }}
            >
              <GithubOutlined style={{ marginRight: 8, flexShrink: 0 }} />
              <span>{GITHUB_URL}</span>
            </Link>
          </div>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16, flexWrap: 'wrap' }}>
            <span style={{ fontWeight: 500, minWidth: 'fit-content', whiteSpace: 'nowrap' }}>许可证:</span>
            <span>Apache License 2.0</span>
          </div>
        </div>
        <Divider style={{ margin: '16px 0' }} />
        <div style={{ textAlign: 'center', color: 'rgba(0, 0, 0, 0.45)' }}>
          <Text type="secondary">
            © 2026 {APP_NAME}. All rights reserved.
          </Text>
          <br />
          <Text type="secondary" style={{ fontSize: 12 }}>
            备案号：浙ICP备2026007675号-1
          </Text>
          <br />
          <Text type="secondary" style={{ fontSize: 12 }}>
            Licensed under the Apache License, Version 2.0
          </Text>
        </div>
      </div>
    </div>
  );
};

export default Footer;
