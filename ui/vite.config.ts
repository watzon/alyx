import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// API server URL - defaults to localhost:8090 for development
// Can be overridden with ALYX_API_URL environment variable
const apiTarget = process.env.ALYX_API_URL || 'http://localhost:8090';

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
			}
		}
	}
});
