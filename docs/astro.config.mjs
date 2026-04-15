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
			components: {
				Hero: './src/components/Hero.astro',
				Banner: './src/components/Banner.astro',
				Head: './src/components/Head.astro',
			},
			defaultLocale: 'root',
			locales: {
				root: { label: 'English', lang: 'en' },
				'zh-cn': { label: '简体中文', lang: 'zh-CN' },
			},
			sidebar: [
				{
					label: 'Getting Started',
					translations: { 'zh-CN': '入门' },
					items: [
						{ label: 'Installation', translations: { 'zh-CN': '安装' }, slug: 'getting-started/installation' },
						{ label: 'Configuration', translations: { 'zh-CN': '配置' }, slug: 'getting-started/configuration' },
						{ label: 'Quick Start', translations: { 'zh-CN': '快速开始' }, slug: 'getting-started/quick-start' },
					],
				},
				{
					label: 'Commands',
					translations: { 'zh-CN': '命令' },
					items: [
						{ label: 'Auth', translations: { 'zh-CN': '身份验证' }, slug: 'commands/auth' },
						{ label: 'Issues', translations: { 'zh-CN': '工单' }, slug: 'commands/issues' },
						{ label: 'Projects', translations: { 'zh-CN': '项目' }, slug: 'commands/projects' },
						{ label: 'Memberships', translations: { 'zh-CN': '成员关系' }, slug: 'commands/memberships' },
						{ label: 'Versions', translations: { 'zh-CN': '版本' }, slug: 'commands/versions' },
						{ label: 'Time Entries', translations: { 'zh-CN': '工时记录' }, slug: 'commands/time' },
						{ label: 'Users', translations: { 'zh-CN': '用户' }, slug: 'commands/users' },
						{ label: 'Groups', translations: { 'zh-CN': '用户组' }, slug: 'commands/groups' },
						{ label: 'Search', translations: { 'zh-CN': '搜索' }, slug: 'commands/search' },
						{ label: 'Wiki', translations: { 'zh-CN': '维基' }, slug: 'commands/wiki' },
						{ label: 'Other', translations: { 'zh-CN': '其他' }, slug: 'commands/other' },
					],
				},
				{
					label: 'Guides',
					translations: { 'zh-CN': '指南' },
					items: [
						{ label: 'AI Agent Integration', translations: { 'zh-CN': 'AI 代理集成' }, slug: 'guides/ai-agents' },
						{ label: 'Output Formats', translations: { 'zh-CN': '输出格式' }, slug: 'guides/output-formats' },
					],
				},
			],
		}),
	],
});
