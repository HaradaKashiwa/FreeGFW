import Index from '@/pages/index'
import Start from '@/pages/start'

export default [{
    path: '/',
    exact: true,
    layout: true,
    trunk: () => Index
  }, {
    path: '/start',
    exact: true,
    layout: false,
    trunk: () => Start
  }
]
  