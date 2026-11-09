import { ref } from 'vue'
import { getPublicDict } from '@/api/public'

export interface DictOption {
  label: string
  value: string
}

// 公开字典只读且带后端缓存，前端再做一次进程内缓存，避免多个页面重复请求。
const cache = new Map<string, DictOption[]>()

/**
 * 业务可配枚举统一走数据字典；接口不可用或字典被停用时回退到内置选项，
 * 保证表单永远能渲染。流程状态机（发布状态、审核状态等）不使用这里。
 */
export function useDictOptions(code: string, fallback: DictOption[] = []) {
  const options = ref<DictOption[]>([...fallback])
  const loading = ref(false)

  async function load(): Promise<void> {
    if (cache.has(code)) {
      options.value = cache.get(code) ?? []
      return
    }
    loading.value = true
    try {
      const resp = await getPublicDict(code)
      const items = (resp.items ?? []).map((it) => ({ label: it.label, value: it.value }))
      options.value = items.length > 0 ? items : [...fallback]
      cache.set(code, options.value)
    } catch {
      options.value = [...fallback]
    } finally {
      loading.value = false
    }
  }

  return { options, loading, load }
}

/** 字典变更后调用，让下拉立即拿到新值。 */
export function invalidateDictOptions(code?: string): void {
  if (code) {
    cache.delete(code)
  } else {
    cache.clear()
  }
}
