module.exports = {
	title: 'Sukhavati Miner Doc',
	description: 'Sukhavati Miner Doc',
	head: [
		['link', { rel: 'icon', href: '/logo.png' }],
	],
	markdown: {
		lineNumbers: true
	},
	serviceWorker: true,
	locales: {
		'/': {
			lang: 'en-US',
			title: 'Sukhavati Docs',
			description: 'Sukhavati Miner Docs'
		},
	},
	themeConfig: {
		logo: '/logo.png',
		lastUpdated: 'lastUpdate', // string | boolean
		locales: {
			'/': {
				selectText: 'Languages',
				label: 'English',
				editLinkText: 'Edit this page on GitHub',
				serviceWorker: {
					updatePopup: {
						message: "New content is available.",
						buttonText: "Refresh"
					}
				},
				algolia: {},
				nav: [
					{ text: 'Home', link: '/' },
					{
						text: 'subject',
						ariaLabel: 'subject',
						items: [
							{ text: 'script', link: '/script_en.md' },
							{ text: 'dev', link: '/dev.md'}
						]
					},
					{ text: 'Github', link: 'https://github.com/Sukhavati-Labs/go-miner/tree/main/docs' },
				],
				sidebar: {
					'/': [
						{
							title: 'miner',
							collapsable: true,
							sidebarDepth: 3,
							children: [
								['dev.md','Dev'],
								['script_en.md', 'script_en']
							]
						}
					],
				}
			},
		}
	}
}	