import react from '@vitejs/plugin-react';
import path from 'node:path';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const apiTarget = env.LIAISON_API_TARGET || 'http://127.0.0.1:8080';

  return {
    plugins: [react()],
    resolve: {
      alias: { '@': path.resolve(__dirname, './src') },
    },
    build: {
      target: 'es2020',
      outDir: 'dist',
      sourcemap: false,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (!id.includes('node_modules')) return undefined;
            if (id.includes('@ant-design/plots') || id.includes('d3-')) return 'vendor-charts';
            if (id.includes('@xterm') || id.includes('guacamole')) return 'vendor-terminal';
            if (id.includes('antd') || id.includes('@ant-design')) return 'vendor-antd';
            return undefined;
          },
        },
      },
    },
    server: {
      port: 8000,
      proxy: {
        '/api': { target: apiTarget, changeOrigin: true, secure: false },
      },
    },
  };
});
