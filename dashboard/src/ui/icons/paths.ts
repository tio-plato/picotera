import type { Component } from 'vue'
import {
  IconActivity,
  IconAlignJustified,
  IconBraces,
  IconCpu,
  IconDatabase,
  IconEdit,
  IconGitBranch,
  IconLink,
  IconList,
  IconPlug,
  IconPlus,
  IconRefresh,
  IconSettings,
  IconTrash,
  IconX,
} from '@tabler/icons-vue'

export type IconName =
  | 'plus'
  | 'edit'
  | 'trash'
  | 'close'
  | 'close-sm'
  | 'link'
  | 'settings'
  | 'cpu'
  | 'plug'
  | 'branch'
  | 'db'
  | 'list'
  | 'lines'
  | 'braces'
  | 'activity'
  | 'refresh'

export const iconComponents: Record<IconName, Component> = {
  plus: IconPlus,
  edit: IconEdit,
  trash: IconTrash,
  close: IconX,
  'close-sm': IconX,
  link: IconLink,
  settings: IconSettings,
  cpu: IconCpu,
  plug: IconPlug,
  branch: IconGitBranch,
  db: IconDatabase,
  list: IconList,
  lines: IconAlignJustified,
  braces: IconBraces,
  activity: IconActivity,
  refresh: IconRefresh,
}
