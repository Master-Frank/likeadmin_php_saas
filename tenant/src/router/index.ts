import { createRouter, createWebHistory, type RouteRecordRaw, RouterView } from 'vue-router'

import { MenuEnum } from '@/enums/appEnums'
import useUserStore from '@/stores/modules/user'
import { isExternal } from '@/utils/validate'

import { constantRoutes, INDEX_ROUTE_NAME, LAYOUT } from './routes'

// 匹配views里面所有的.vue文件，动态引入
const modules = import.meta.glob('/src/views/**/*.vue')

//
export function getModulesKey() {
    return Object.keys(modules).map((item) => item.replace('/src/views/', '').replace('.vue', ''))
}

// 过滤路由所需要的数据
export function filterAsyncRoutes(routes: any[], firstRoute = true, parentPath = '') {
    return routes.map((route) => {
        const routeRecord = createRouteRecord(route, firstRoute)
        const fullPath =
            isExternal(routeRecord.path) || routeRecord.path.startsWith('/')
                ? routeRecord.path
                : joinRoutePath(parentPath, routeRecord.path)
        if (route.children != null && route.children && route.children.length) {
            routeRecord.children = filterAsyncRoutes(route.children, false, fullPath)
            // 目录本身没有页面。/permission、/consumer 直接打开时跳到第一个子菜单，
            // 避免只剩一个空的 router-view。
            if (route.type == MenuEnum.CATALOGUE && !isExternal(fullPath)) {
                const target = firstMenuPath(routeRecord.children, fullPath)
                if (target) {
                    routeRecord.redirect = target
                }
            }
        }
        return routeRecord
    })
}

function joinRoutePath(parent: string, path: string) {
    if (!path) {
        return parent || '/'
    }
    if (path.startsWith('/')) {
        return path.replace(/\/{2,}/g, '/')
    }
    const base = parent.endsWith('/') ? parent.slice(0, -1) : parent
    return `${base}/${path}`.replace(/\/{2,}/g, '/')
}

// 目录下第一个可见菜单的绝对路径，例如 /permission/menu、/consumer/lists。
function firstMenuPath(children: RouteRecordRaw[] | undefined, parentPath: string): string | undefined {
    if (!children) {
        return
    }
    for (const child of children) {
        if (child.meta?.hidden || isExternal(child.path)) {
            continue
        }
        const fullPath = joinRoutePath(parentPath, child.path)
        if (child.meta?.type == MenuEnum.MENU) {
            return fullPath
        }
        const nested = firstMenuPath(child.children, fullPath)
        if (nested) {
            return nested
        }
    }
    return undefined
}

// 创建一条路由记录
export function createRouteRecord(route: any, firstRoute: boolean): RouteRecordRaw {
    //@ts-ignore
    const routeRecord: RouteRecordRaw = {
        path: isExternal(route.paths) ? route.paths : firstRoute ? `/${route.paths}` : route.paths,
        name: Symbol(route.paths),
        meta: {
            hidden: !route.is_show,
            keepAlive: !!route.is_cache,
            title: route.name,
            perms: route.perms,
            query: route.params,
            icon: route.icon,
            type: route.type,
            activeMenu: route.selected
        }
    }
    switch (route.type) {
        case MenuEnum.CATALOGUE:
            routeRecord.component = firstRoute ? LAYOUT : RouterView
            if (!route.children) {
                routeRecord.component = RouterView
            }
            break
        case MenuEnum.MENU:
            routeRecord.component = loadRouteView(route.component)
            break
    }
    return routeRecord
}

// 动态加载组件
export function loadRouteView(component: string) {
    try {
        const key = Object.keys(modules).find((key) => {
            return key.includes(`/${component}.vue`)
        })
        if (key) {
            return modules[key]
        }
        throw Error(`找不到组件${component}，请确保组件路径正确`)
    } catch (error) {
        console.error(error)
        return RouterView
    }
}

// 找到第一个有效的路由
export function findFirstValidRoute(routes: RouteRecordRaw[]): string | undefined {
    for (const route of routes) {
        if (route.meta?.type == MenuEnum.MENU && !route.meta?.hidden && !isExternal(route.path)) {
            return route.name as string
        }
        if (route.children) {
            const name = findFirstValidRoute(route.children)
            if (name) {
                return name
            }
        }
    }
}
//通过权限字符查询路由路径
export function getRoutePath(perms: string) {
    const routerObj = useRouter() || router
    return routerObj.getRoutes().find((item) => item.meta?.perms == perms)?.path || ''
}

// 重置路由
export function resetRouter() {
    router.removeRoute(INDEX_ROUTE_NAME)
    const { routes } = useUserStore()
    routes.forEach((route) => {
        const name = route.name
        if (name && router.hasRoute(name)) {
            router.removeRoute(name)
        }
    })
}

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes: constantRoutes
})

export default router
