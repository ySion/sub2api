<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
          {{ t('admin.permissions.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.permissions.description') }}
        </p>
      </div>
      <button class="btn btn-secondary" :disabled="loading" @click="loadUsers">
        {{ t('common.refresh') }}
      </button>
    </div>

    <div class="card overflow-hidden">
      <div v-if="loading" class="p-6 text-sm text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}
      </div>
      <div v-else-if="users.length === 0" class="p-6 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.permissions.empty') }}
      </div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-800/60">
            <tr>
              <th class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('admin.permissions.user') }}
              </th>
              <th class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('admin.permissions.currentRole') }}
              </th>
              <th class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('admin.permissions.newRole') }}
              </th>
              <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('common.actions') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-900">
            <tr v-for="user in users" :key="user.id">
              <td class="px-4 py-3">
                <div class="font-medium text-gray-900 dark:text-white">
                  {{ user.username || user.email }}
                </div>
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  {{ user.email }} · ID {{ user.id }}
                </div>
              </td>
              <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                {{ roleLabel(user.role) }}
              </td>
              <td class="px-4 py-3">
                <select
                  v-model="selectedRoles[user.id]"
                  class="input max-w-[180px]"
                  :disabled="isSelf(user) || savingId === user.id"
                >
                  <option v-for="role in roleOptions" :key="role" :value="role">
                    {{ roleLabel(role) }}
                  </option>
                </select>
                <p v-if="isSelf(user)" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
                  {{ t('admin.permissions.selfChangeHint') }}
                </p>
              </td>
              <td class="px-4 py-3 text-right">
                <button
                  class="btn btn-primary btn-sm"
                  :disabled="isSelf(user) || savingId === user.id || selectedRoles[user.id] === user.role"
                  @click="saveRole(user)"
                >
                  {{ savingId === user.id ? t('common.saving') : t('common.save') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { usersAPI } from '@/api/admin'
import { useAppStore, useAuthStore } from '@/stores'
import type { AdminUser, UserRole } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const roleOptions: UserRole[] = ['user', 'operator', 'admin']
const users = ref<AdminUser[]>([])
const selectedRoles = ref<Record<number, UserRole>>({})
const loading = ref(false)
const savingId = ref<number | null>(null)

function roleLabel(role: UserRole): string {
  if (role === 'admin') return t('profile.administrator')
  if (role === 'operator') return t('profile.operator')
  return t('profile.user')
}

function isSelf(user: AdminUser): boolean {
  return authStore.user?.id === user.id
}

async function loadUsers(): Promise<void> {
  loading.value = true
  try {
    const result = await usersAPI.list(1, 100, {
      include_subscriptions: false,
      sort_by: 'created_at',
      sort_order: 'desc'
    })
    users.value = result.items
    selectedRoles.value = Object.fromEntries(result.items.map((user) => [user.id, user.role]))
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || t('admin.permissions.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function saveRole(user: AdminUser): Promise<void> {
  const role = selectedRoles.value[user.id]
  if (!role || role === user.role) return
  savingId.value = user.id
  try {
    const updated = await usersAPI.updateRole(user.id, role)
    users.value = users.value.map((item) => item.id === updated.id ? updated : item)
    selectedRoles.value[updated.id] = updated.role
    appStore.showSuccess(t('admin.permissions.saveSuccess'))
  } catch (error: any) {
    selectedRoles.value[user.id] = user.role
    appStore.showError(error?.response?.data?.message || t('admin.permissions.saveFailed'))
  } finally {
    savingId.value = null
  }
}

onMounted(() => {
  loadUsers()
})
</script>
