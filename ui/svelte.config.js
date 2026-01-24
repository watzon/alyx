import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	kit: {
		adapter: adapter({
			pages: 'build',
			assets: 'build',
			fallback: 'index.html',
			precompress: false,
			strict: true
		}),
		paths: {
			base: '/_admin'
		},
		alias: {
			$components: 'src/lib/components',
			$ui: 'src/lib/components/ui'
		},
		prerender: {
			handleUnseenRoutes: 'ignore'
		}
	}
};

export default config;
