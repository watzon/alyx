import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig, loadEnv } from 'vite';

const apiTarget = loadEnv('', process.cwd(), '').ALYX_API_URL || 'http://localhost:8090';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': {
				target: apiTarget,
				changeOrigin: true
			},
			'/internal': {
				target: apiTarget,
				changeOrigin: true
			},
			'/metrics': {
				target: apiTarget,
				changeOrigin: true
			},
			'/health': {
				target: apiTarget,
				changeOrigin: true
			}
		}
	}
});
