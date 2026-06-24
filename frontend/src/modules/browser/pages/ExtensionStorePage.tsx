import { useState } from 'react'
import { Search, ExternalLink, Star, Users, Package } from 'lucide-react'
import { Badge, Button, Card, Input, toast } from '../../../shared/components'

// 常用扩展列表（预置）
const POPULAR_EXTENSIONS = [
  {
    id: 'cjpalhdlnbpafiamejdnhcphjbkeiagm',
    name: 'uBlock Origin',
    description: '高效的请求过滤工具：占用极低的内存和CPU',
    category: '生产工具',
    rating: 4.8,
    users: '10,000,000+',
    icon: 'https://lh3.googleusercontent.com/KJ_u7SX0l6HY7hgv1Uy1v8v7OqU7J7v_-M8v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'nngceckbapebfimnlniiiahkandclblb',
    name: 'Bitwarden',
    description: '适合个人、团队与商业组织使用的安全且免费的密码管理器',
    category: '生产工具',
    rating: 4.7,
    users: '3,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'eimadpbcbfnmbkopoojfekhnkhdbieeh',
    name: 'Dark Reader',
    description: '适用于任何网站的黑暗主题。关爱眼睛，就使用Dark Reader进行夜间和日间浏览',
    category: '无障碍',
    rating: 4.6,
    users: '5,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'dhdgffkkebhmkfjojejmpbldmpobfkfo',
    name: 'Tampermonkey',
    description: '世界上最流行的用户脚本管理器',
    category: '生产工具',
    rating: 4.7,
    users: '10,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'padekgcemlokbadohgkifijomclgjgif',
    name: 'Proxy SwitchyOmega',
    description: '轻松快捷地管理和切换多个代理设置',
    category: '生产工具',
    rating: 4.6,
    users: '2,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'nkbihfbeogaeaoehlefnkodbefgpgknn',
    name: 'MetaMask',
    description: '以太坊浏览器扩展',
    category: '生产工具',
    rating: 4.2,
    users: '10,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'hdokiejnpimakedhajhdlcegeplioahd',
    name: 'LastPass',
    description: '免费密码管理器',
    category: '生产工具',
    rating: 4.5,
    users: '8,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'fhbjgbiflinjbdggehcddcbncdddomop',
    name: 'Postman',
    description: 'API测试工具',
    category: '开发者工具',
    rating: 4.3,
    users: '3,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'gighmmpiobklfepjocnamgkkbiglidom',
    name: 'AdBlock',
    description: 'Chrome上最受欢迎的广告拦截工具，拥有超过 4 亿次下载',
    category: '生产工具',
    rating: 4.7,
    users: '10,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'oldceeleldhonbafppcapldpdifcinji',
    name: 'Grammarly',
    description: '书写助手',
    category: '生产工具',
    rating: 4.5,
    users: '10,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'mooikfkahbdckldjjndioackbalphokd',
    name: 'Selenium IDE',
    description: 'Web自动化测试工具',
    category: '开发者工具',
    rating: 4.1,
    users: '1,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
  {
    id: 'bfbameneiokkgbdmiekhjnmfkcnldhhm',
    name: 'Web Developer',
    description: '添加一个包含各种Web开发工具的工具栏按钮',
    category: '开发者工具',
    rating: 4.4,
    users: '1,000,000+',
    icon: 'https://lh3.googleusercontent.com/v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v=w128-h128',
  },
]

const CATEGORIES = ['全部', '生产工具', '开发者工具', '无障碍', '购物', '娱乐']

export function ExtensionStorePage() {
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedCategory, setSelectedCategory] = useState('全部')

  const filteredExtensions = POPULAR_EXTENSIONS.filter(ext => {
    const matchSearch = ext.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      ext.description.toLowerCase().includes(searchQuery.toLowerCase())
    const matchCategory = selectedCategory === '全部' || ext.category === selectedCategory
    return matchSearch && matchCategory
  })

  const handleOpenInWebStore = (extensionId: string) => {
    const url = `https://chrome.google.com/webstore/detail/${extensionId}`
    window.open(url, '_blank')
  }

  const handleCopyId = (extensionId: string) => {
    navigator.clipboard.writeText(extensionId)
    toast.success('扩展ID已复制到剪贴板')
  }

  return (
    <div className="overflow-auto p-5 space-y-5 animate-fade-in h-full">
      {/* 页头 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">扩展商店</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">
            浏览和安装常用Chrome扩展
          </p>
        </div>
      </div>

      {/* 使用说明 */}
      <Card padding="md">
        <div className="space-y-3">
          <h3 className="text-sm font-medium text-[var(--color-text-primary)]">📦 如何安装扩展</h3>
          <div className="space-y-2 text-sm text-[var(--color-text-secondary)]">
            <p><strong>方法一：从Chrome Web Store安装（推荐）</strong></p>
            <ol className="list-decimal list-inside space-y-1 ml-2">
              <li>点击扩展卡片的"在商店中打开"按钮</li>
              <li>在Chrome Web Store页面点击"添加到Chrome"</li>
              <li>打开Chrome扩展页面 <code className="px-1 py-0.5 bg-[var(--color-bg-muted)] rounded text-xs font-mono">chrome://extensions/</code></li>
              <li>开启"开发者模式"，找到扩展的安装路径</li>
              <li>复制路径，回到"扩展管理"页面添加</li>
            </ol>

            <p className="pt-2"><strong>方法二：手动下载并解压</strong></p>
            <ol className="list-decimal list-inside space-y-1 ml-2">
              <li>使用第三方工具下载crx文件（如：CRX Extractor）</li>
              <li>解压crx文件到本地文件夹</li>
              <li>在"扩展管理"页面添加解压后的文件夹路径</li>
            </ol>
          </div>
        </div>
      </Card>

      {/* 搜索和分类 */}
      <div className="flex flex-col md:flex-row gap-4">
        <div className="flex-1">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-[var(--color-text-muted)]" />
            <Input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="搜索扩展..."
              className="pl-10"
            />
          </div>
        </div>
        <div className="flex gap-2 overflow-auto">
          {CATEGORIES.map(cat => (
            <Button
              key={cat}
              size="sm"
              variant={selectedCategory === cat ? 'primary' : 'secondary'}
              onClick={() => setSelectedCategory(cat)}
            >
              {cat}
            </Button>
          ))}
        </div>
      </div>

      {/* 扩展列表 */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {filteredExtensions.map(ext => (
          <Card key={ext.id} padding="md" className="hover:shadow-lg transition-shadow">
            <div className="space-y-3">
              {/* 头部：图标和名称 */}
              <div className="flex items-start gap-3">
                <div className="w-12 h-12 rounded-lg bg-blue-50 dark:bg-blue-900/30 flex items-center justify-center flex-shrink-0">
                  <Package className="w-6 h-6 text-blue-600" />
                </div>
                <div className="flex-1 min-w-0">
                  <h3 className="font-medium text-[var(--color-text-primary)] truncate">
                    {ext.name}
                  </h3>
                  <Badge variant="default" className="mt-1">{ext.category}</Badge>
                </div>
              </div>

              {/* 描述 */}
              <p className="text-sm text-[var(--color-text-secondary)] line-clamp-2 min-h-[2.5rem]">
                {ext.description}
              </p>

              {/* 统计信息 */}
              <div className="flex items-center gap-4 text-xs text-[var(--color-text-muted)]">
                <div className="flex items-center gap-1">
                  <Star className="w-3.5 h-3.5 text-yellow-500 fill-yellow-500" />
                  <span>{ext.rating}</span>
                </div>
                <div className="flex items-center gap-1">
                  <Users className="w-3.5 h-3.5" />
                  <span>{ext.users}</span>
                </div>
              </div>

              {/* 操作按钮 */}
              <div className="flex gap-2 pt-2">
                <Button
                  size="sm"
                  variant="secondary"
                  className="flex-1"
                  onClick={() => handleOpenInWebStore(ext.id)}
                >
                  <ExternalLink className="w-3.5 h-3.5" />
                  在商店中打开
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => handleCopyId(ext.id)}
                  title="复制扩展ID"
                >
                  ID
                </Button>
              </div>
            </div>
          </Card>
        ))}
      </div>

      {/* 空状态 */}
      {filteredExtensions.length === 0 && (
        <div className="py-20 text-center">
          <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gray-50 dark:bg-gray-900/30 flex items-center justify-center">
            <Search className="w-8 h-8 text-gray-400" />
          </div>
          <h3 className="text-base font-semibold text-[var(--color-text-primary)] mb-1">
            未找到扩展
          </h3>
          <p className="text-sm text-[var(--color-text-muted)]">
            尝试其他搜索关键词或分类
          </p>
        </div>
      )}

      {/* 底部提示 */}
      <Card padding="md">
        <div className="space-y-2">
          <h3 className="text-sm font-medium text-[var(--color-text-primary)]">💡 提示</h3>
          <ul className="text-sm text-[var(--color-text-secondary)] space-y-1">
            <li>• 这些扩展来自Chrome Web Store，安全可靠</li>
            <li>• 某些扩展可能需要特定的Chrome版本支持</li>
            <li>• 安装后需要在"扩展管理"中添加路径才能使用</li>
            <li>• 部分扩展可能需要登录账号才能使用完整功能</li>
          </ul>
        </div>
      </Card>
    </div>
  )
}
