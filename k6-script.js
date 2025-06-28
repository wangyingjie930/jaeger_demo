import http from 'k6/http';
import { check, sleep, fail } from 'k6';
import { Trend } from 'k6/metrics';

// 1. 定义自定义趋势指标
const vipOrderTrend = new Trend('vip_order_response_time');
const normalOrderTrend = new Trend('normal_order_response_time');

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 10 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    // 我们的性能目标：95%的请求必须在800ms内完成
    'http_req_duration': ['p(95)<800'],
    // 我们的可靠性目标：请求失败率必须低于10%
    'http_req_failed': ['rate<0.1'],
    // 添加自定义指标的阈值监控
    'vip_order_response_time': ['p(95)<600'], // VIP用户应该有更快的响应时间
    'normal_order_response_time': ['p(95)<800'],
  },
  // 2. 关键修复：明确告诉k6在摘要中展示这些统计值
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};

const API_BASE_URL = __ENV.API_BASE_URL || 'http://localhost:8080';

// 3. 添加随机数据生成函数
function getRandomUserID(isVip = false) {
  const prefix = isVip ? 'vip' : 'normal';
  const randomNum = Math.floor(Math.random() * 10000);
  return `user-${prefix}-${randomNum}`;
}

function getRandomItems() {
  const allItems = ['item-a', 'item-b', 'item-c', 'item-d', 'item-e'];
  const itemCount = Math.floor(Math.random() * 3) + 1; // 1-3个商品
  const selectedItems = [];
  
  for (let i = 0; i < itemCount; i++) {
    const randomItem = allItems[Math.floor(Math.random() * allItems.length)];
    if (!selectedItems.includes(randomItem)) {
      selectedItems.push(randomItem);
    }
  }
  
  return selectedItems.join(',');
}

export default function () {
  // --- 场景 1: VIP 用户订单 ---
  const vipUserID = getRandomUserID(true);
  const vipItems = getRandomItems();
  const vipUrl = `${API_BASE_URL}/create_complex_order?userID=${vipUserID}&is_vip=true&items=${vipItems}`;
  
  const vipRes = http.post(vipUrl, null, {
    tags: { name: 'CreateComplexOrder-VIP' },
    timeout: '10s', // 添加超时设置
  });

  const vipCheck = check(vipRes, {
    'VIP order creation successful (status 200)': (r) => r && r.status === 200,
  });
    
  if (!vipCheck) {
    fail(`VIP order request failed. Status: ${vipRes.status}, Body: ${vipRes.body}`);
  }

  // 4. 简化响应时间记录逻辑 - 直接使用timings.duration
  if (vipRes.status === 200) {
    vipOrderTrend.add(vipRes.timings.duration);
  }

  // --- 场景 2: 普通用户订单 ---
  const normalUserID = getRandomUserID(false);
  const normalItems = getRandomItems();
  const normalUrl = `${API_BASE_URL}/create_complex_order?userID=${normalUserID}&is_vip=false&items=${normalItems}`;
  
  const normalRes = http.post(normalUrl, null, {
    tags: { name: 'CreateComplexOrder-Normal' },
    timeout: '10s', // 添加超时设置
  });

  const normalCheck = check(normalRes, {
    'Normal order creation successful (status 200)': (r) => r && r.status === 200,
  });
    
  if (!normalCheck) {
    fail(`Normal order request failed. Status: ${normalRes.status}, Body: ${normalRes.body}`);
  }

  if (normalRes.status === 200) {
    normalOrderTrend.add(normalRes.timings.duration);
  }

  // 5. 添加随机等待时间，模拟真实用户行为
  sleep(Math.random() * 2 + 0.5); // 0.5-2.5秒随机等待
}