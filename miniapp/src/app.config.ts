export default defineAppConfig({
  pages: [
    'pages/home/index',
    'pages/competitors/index',
    'pages/compare/index',
    'pages/messages/index',
    'pages/login/index'
  ],
  tabBar: {
    color: '#86909c',
    selectedColor: '#4f46e5',
    backgroundColor: '#ffffff',
    borderStyle: 'white',
    list: [
      { pagePath: 'pages/home/index', text: '总览' },
      { pagePath: 'pages/competitors/index', text: '竞品' },
      { pagePath: 'pages/compare/index', text: '归因' },
      { pagePath: 'pages/messages/index', text: '消息' }
    ]
  },
  window: {
    backgroundTextStyle: 'light',
    navigationBarBackgroundColor: '#ffffff',
    navigationBarTitleText: 'LinkGeo',
    navigationBarTextStyle: 'black',
    backgroundColor: '#f5f6fa'
  }
})
