import { defineStore } from 'pinia'
import { getConfig } from '~~/api/app'

interface AppSate {
    config: Record<string, any>
}
export const useAppStore = defineStore({
    id: 'appStore',
    state: (): AppSate => ({
        config: {}
    }),
    getters: {
        getImageUrl: (state) => (url: string) => {
            if (!url) {
                return ''
            }
            if (/^https?:\/\//i.test(url)) {
                return url
            }
            const domain = String(state.config.domain || '').replace(/\/+$/, '')
            const path = String(url).replace(/^\/+/, '')
            if (!domain) {
                return '/' + path
            }
            return `${domain}/${path}`
        },
        getWebsiteConfig: (state) => state.config.website || {},
        getLoginConfig: (state) => state.config.login || {},
        getCopyrightConfig: (state) => state.config.copyright || [],
        getQrcodeConfig: (state) => state.config.qrcode || {},
        getAdminUrl: (state) => state.config.admin_url,
        getSiteStatistics: (state) => state.config.siteStatistics || {}
    },
    actions: {
        async getConfig() {
            const config = await getConfig()
            this.config = config
        },
        setConfig(config: Record<string, any>) {
            this.config = config
        }
    }
})
