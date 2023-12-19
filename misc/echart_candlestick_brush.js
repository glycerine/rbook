import * as echarts from 'echarts/core';

import {
    ToolboxComponent,
    TooltipComponent,
    GridComponent,
    VisualMapComponent,
    LegendComponent,
    BrushComponent,
    DataZoomComponent
} from 'echarts/components';

import { CandlestickChart, LineChart, BarChart } from 'echarts/charts';
import { UniversalTransition } from 'echarts/features';
import { CanvasRenderer } from 'echarts/renderers';

echarts.use([
    ToolboxComponent,
    TooltipComponent,
    GridComponent,
    VisualMapComponent,
    LegendComponent,
    BrushComponent,
    DataZoomComponent,
    CandlestickChart,
    LineChart,
    BarChart,
    CanvasRenderer,
    UniversalTransition
]);

var ROOT_PATH = 'https://echarts.apache.org/examples';

var chartDom = document.getElementById('main');
var myChart = echarts.init(chartDom);
var option;

const upColor = '#00da3c';
const downColor = '#ec0000';
function splitData(rawData) {
    let categoryData = [];
    let values = [];
    let volumes = [];
    for (let i = 0; i < rawData.length; i++) {
	categoryData.push(rawData[i].splice(0, 1)[0]);
	values.push(rawData[i]);
	volumes.push([i, rawData[i][4], rawData[i][0] > rawData[i][1] ? 1 : -1]);
    }
    return {
	categoryData: categoryData,
	values: values,
	volumes: volumes
    };
}
function calculateMA(dayCount, data) {
    var result = [];
    for (var i = 0, len = data.values.length; i < len; i++) {
	if (i < dayCount) {
	    result.push('-');
	    continue;
	}
	var sum = 0;
	for (var j = 0; j < dayCount; j++) {
	    sum += data.values[i - j][1];
	}
	result.push(+(sum / dayCount).toFixed(3));
    }
    return result;
}
$.get(ROOT_PATH + '/data/asset/data/stock-DJI.json', function (rawData) {
    var data = splitData(rawData);
    myChart.setOption(
	(option = {
	    animation: false,
	    legend: {
		bottom: 10,
		left: 'center',
		data: ['Dow-Jones index', 'MA5', 'MA10', 'MA20', 'MA30']
	    },
	    tooltip: {
		trigger: 'axis',
		axisPointer: {
		    type: 'cross'
		},
		borderWidth: 1,
		borderColor: '#ccc',
		padding: 10,
		textStyle: {
		    color: '#000'
		},
		position: function (pos, params, el, elRect, size) {
		    const obj = {
			top: 10
		    };
		    obj[['left', 'right'][+(pos[0] < size.viewSize[0] / 2)]] = 30;
		    return obj;
		}
		// extraCssText: 'width: 170px'
	    },
	    axisPointer: {
		link: [
		    {
			xAxisIndex: 'all'
		    }
		}
	    ]
	});
    });

option && myChart.setOption(option);
      
