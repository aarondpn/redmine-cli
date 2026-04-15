import { defineCollection, z } from 'astro:content';
import { docsLoader, i18nLoader } from '@astrojs/starlight/loaders';
import { docsSchema, i18nSchema } from '@astrojs/starlight/schema';

export const collections = {
	docs: defineCollection({ loader: docsLoader(), schema: docsSchema() }),
	i18n: defineCollection({
		loader: i18nLoader(),
		schema: i18nSchema({
			extend: z.object({
				'contributors.roster': z.string(),
				'contributors.commits': z.string(),
				'contributors.count_one': z.string(),
				'contributors.count_other': z.string(),
				'contributors.commitsUnit': z.string(),
				'contributors.emptyBefore': z.string(),
				'contributors.emptyLink': z.string(),
				'hero.statusAriaLabel': z.string(),
				'hero.ver': z.string(),
				'hero.build': z.string(),
				'hero.lic': z.string(),
				'hero.stars': z.string(),
				'hero.sys': z.string(),
				'hero.fallbackTagline': z.string(),
				'terminal.subject1': z.string(),
				'terminal.subject2': z.string(),
				'terminal.subject3': z.string(),
				'terminal.notes': z.string(),
				'terminal.closed': z.string(),
				'capability.issues.title': z.string(),
				'capability.issues.desc': z.string(),
				'capability.time.title': z.string(),
				'capability.time.desc': z.string(),
				'capability.projects.title': z.string(),
				'capability.projects.desc': z.string(),
				'capability.names.title': z.string(),
				'capability.names.desc': z.string(),
				'capability.names.anchor': z.string(),
				'capability.formats.title': z.string(),
				'capability.formats.desc': z.string(),
				'capability.agents.title': z.string(),
				'capability.agents.desc': z.string(),
				'banner.translationNotice': z.string(),
				'banner.dismissLabel': z.string(),
			}),
		}),
	}),
};
