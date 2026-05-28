import { use, type ComposeOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import {
  LineChart,
  type LineSeriesOption,
  PieChart,
  type PieSeriesOption,
  SankeyChart,
  type SankeySeriesOption,
  CustomChart,
  type CustomSeriesOption,
  BoxplotChart,
  type BoxplotSeriesOption,
} from 'echarts/charts'
import {
  GridComponent,
  type GridComponentOption,
  TooltipComponent,
  type TooltipComponentOption,
  DatasetComponent,
  type DatasetComponentOption,
  GraphicComponent,
  type GraphicComponentOption,
} from 'echarts/components'

use([
  CanvasRenderer,
  LineChart,
  PieChart,
  SankeyChart,
  CustomChart,
  BoxplotChart,
  GridComponent,
  TooltipComponent,
  DatasetComponent,
  GraphicComponent,
])

export type EChartsOption = ComposeOption<
  | LineSeriesOption
  | PieSeriesOption
  | SankeySeriesOption
  | CustomSeriesOption
  | BoxplotSeriesOption
  | GridComponentOption
  | TooltipComponentOption
  | DatasetComponentOption
  | GraphicComponentOption
>
