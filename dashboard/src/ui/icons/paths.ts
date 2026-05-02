import type { Component } from 'vue'
import {
  IconActivity,
  IconAlignJustified,
  IconBraces,
  IconCheck,
  IconChevronDown,
  IconCloudDownload,
  IconCloudUpload,
  IconCpu,
  IconDatabase,
  IconEdit,
  IconEye,
  IconEyeOff,
  IconGitBranch,
  IconLink,
  IconList,
  IconLoader2,
  IconPlug,
  IconPlus,
  IconRefresh,
  IconSearch,
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
  | 'chevron-down'
  | 'search'
  | 'cloud-download'
  | 'cloud-upload'
  | 'loader'
  | 'check'
  | 'eye'
  | 'eye-off'

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
  'chevron-down': IconChevronDown,
  search: IconSearch,
  'cloud-download': IconCloudDownload,
  'cloud-upload': IconCloudUpload,
  loader: IconLoader2,
  check: IconCheck,
  eye: IconEye,
  'eye-off': IconEyeOff,
}
