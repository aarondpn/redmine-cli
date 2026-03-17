// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://aarondpn.github.io',
	base: '/redmine-cli',
	integrations: [
		starlight({
			title: 'redmine-cli',
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/aarondpn/redmine-cli' }],
			customCss: ['./src/styles/custom.css'],
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Installation', slug: 'getting-started/installation' },
						{ label: 'Configuration', slug: 'getting-started/configuration' },
						{ label: 'Quick Start', slug: 'getting-started/quick-start' },
					],
				},
				{
					label: 'Commands',
					items: [
						{ label: 'Issues', slug: 'commands/issues' },
						{ label: 'Projects', slug: 'commands/projects' },
						{ label: 'Time Entries', slug: 'commands/time' },
						{ label: 'Users', slug: 'commands/users' },
						{ label: 'Groups', slug: 'commands/groups' },
						{ label: 'Search', slug: 'commands/search' },
						{ label: 'Versions', slug: 'commands/versions' },
						{ label: 'Other', slug: 'commands/other' },
					],
				},
				{
					label: 'Guides',
					items: [
						{ label: 'AI Agent Integration', slug: 'guides/ai-agents' },
						{ label: 'Output Formats', slug: 'guides/output-formats' },
					],
				},
			],
		}),
	],
});
