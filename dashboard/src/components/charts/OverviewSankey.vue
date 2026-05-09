<script setup lang="ts">
import { computed } from 'vue'
import { VisSingleContainer, VisSankey, VisTooltip, VisSankeySelectors } from '@unovis/vue'
import type { SankeyNode as UnovisSankeyNode, SankeyLink as UnovisSankeyLink } from '@unovis/ts'
import { groupColor } from './colors'

export interface SankeyNode {
  id: string
  label: string
  layer: number
}

export interface SankeyLink {
  source: string
  target: string
  value: number
}

const props = defineProps<{
  nodes: SankeyNode[]
  links: SankeyLink[]
  valueFormat?: (value: number) => string
}>()

const graphData = computed(() => ({
  nodes: props.nodes,
  links: props.links,
}))

const nodeTotals = computed(() => {
  const incoming = new Map<string, number>()
  const outgoing = new Map<string, number>()
  for (const link of props.links) {
    incoming.set(link.target, (incoming.get(link.target) ?? 0) + link.value)
    outgoing.set(link.source, (outgoing.get(link.source) ?? 0) + link.value)
  }
  return { incoming, outgoing }
})

const nodeColor = (n: UnovisSankeyNode<SankeyNode, SankeyLink>) => {
  if (n.id.startsWith('__other__')) return 'var(--color-ink-faint)'
  return groupColor(n.layer)
}

const nodeLabel = (n: UnovisSankeyNode<SankeyNode, SankeyLink>) => n.label

const fmt = (v: number) => (props.valueFormat ? props.valueFormat(v) : String(v))

function escape(s: string) {
  return s.replace(/[&<>"']/g, (c) =>
    c === '&'
      ? '&amp;'
      : c === '<'
        ? '&lt;'
        : c === '>'
          ? '&gt;'
          : c === '"'
            ? '&quot;'
            : '&#39;',
  )
}

const tooltipTriggers = computed(() => ({
  [VisSankeySelectors.link]: (d: unknown) => {
    const link = d as UnovisSankeyLink<SankeyNode, SankeyLink> | null
    if (!link) return ''
    const sourceName = (link.source as UnovisSankeyNode<SankeyNode, SankeyLink>).label
    const targetName = (link.target as UnovisSankeyNode<SankeyNode, SankeyLink>).label
    return `<div class="text-xs"><div class="font-medium">${escape(sourceName)} → ${escape(targetName)}</div><div class="mono tabular text-ink-muted">${fmt(link.value)}</div></div>`
  },
  [VisSankeySelectors.node]: (d: unknown) => {
    const node = d as UnovisSankeyNode<SankeyNode, SankeyLink> | null
    if (!node) return ''
    const total = Math.max(
      nodeTotals.value.incoming.get(node.id) ?? 0,
      nodeTotals.value.outgoing.get(node.id) ?? 0,
    )
    return `<div class="text-xs"><div class="font-medium">${escape(node.label)}</div><div class="mono tabular text-ink-muted">${fmt(total)}</div></div>`
  },
}))
</script>

<template>
  <div class="h-72">
    <VisSingleContainer :data="graphData" :height="288">
      <VisSankey
        :node-color="nodeColor"
        :label="nodeLabel"
        :node-padding="14"
        :node-width="14"
      />
      <VisTooltip :triggers="tooltipTriggers" />
    </VisSingleContainer>
  </div>
</template>
