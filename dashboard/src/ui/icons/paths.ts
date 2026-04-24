import type { Component } from 'vue'
import {
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
}
