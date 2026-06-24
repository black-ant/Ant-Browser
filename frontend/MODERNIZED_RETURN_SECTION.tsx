  return (
    <div className="flex flex-col h-screen bg-[var(--color-bg-base)]">
      {confirmDialog}

      {/* 顶部标题栏 */}
      <header className="flex-shrink-0 px-6 py-4 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)]">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">实例管理</h1>
            <p className="text-sm text-[var(--color-text-muted)] mt-0.5">
              共 {profiles.length} 个实例
              {filteredProfiles.length !== profiles.length && (
                <span className="text-[var(--color-accent)]"> · 筛选后 {filteredProfiles.length} 个</span>
              )}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={handleOpenSettings}>
              <Sliders className="w-4 h-4" />
              基础配置
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => { setCdKey(''); setExpandModalOpen(true); loadQuota() }}
            >
              <Gift className="w-4 h-4" />
              扩容
            </Button>
          </div>
        </div>
      </header>

      {/* 状态概览 */}
      <div className="flex-shrink-0 px-6 py-4 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)]">
        <StatusOverview {...statusStats} />
      </div>

      {/* 工具栏 */}
      <EnhancedToolbar
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        onRefresh={handleRefresh}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        filterActive={filterPanelOpen}
        onToggleFilter={() => setFilterPanelOpen(prev => !prev)}
        refreshing={refreshing}
      />

      {/* 筛选面板（可折叠）*/}
      {filterPanelOpen && (
        <div className="flex-shrink-0 px-6 py-4 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)] animate-fade-in">
          <InstanceFilterBar
            filters={filters}
            onChange={setFilters}
            proxies={proxies}
            cores={cores}
            allTags={allTags}
            groups={groups}
          />
        </div>
      )}

      {/* 批量操作工具栏 */}
      {selectedIds.size > 0 && (
        <div className="flex-shrink-0 px-6 py-3 bg-[var(--color-bg-base)]">
          <EnhancedBatchToolbar
            selectedCount={selectedIds.size}
            totalCount={filteredProfiles.length}
            onSelectAll={handleSelectAll}
            onDeselectAll={handleDeselectAll}
            onBatchStart={handleBatchStart}
            onBatchStop={handleBatchStop}
            onBatchDelete={handleBatchDelete}
            batchLoading={batchLoading}
          />
        </div>
      )}

      {/* 主内容区 - 表格 */}
      <main className="flex-1 overflow-hidden">
        <div className="h-full overflow-auto px-6 py-4">
          {loading ? (
            <div className="flex items-center justify-center h-full">
              <div className="flex flex-col items-center gap-3">
                <div className="w-12 h-12 rounded-full bg-[var(--color-bg-muted)] flex items-center justify-center">
                  <div className="w-6 h-6 border-3 border-[var(--color-accent)] border-t-transparent rounded-full animate-spin" />
                </div>
                <span className="text-sm font-medium text-[var(--color-text-secondary)]">加载中...</span>
              </div>
            </div>
          ) : filteredProfiles.length === 0 ? (
            <div className="flex items-center justify-center h-full">
              <div className="flex flex-col items-center gap-4 max-w-md text-center">
                <div className="w-16 h-16 rounded-2xl bg-[var(--color-bg-muted)] flex items-center justify-center">
                  <FileText className="w-8 h-8 text-[var(--color-text-muted)]" />
                </div>
                <div>
                  <h3 className="text-base font-semibold text-[var(--color-text-primary)] mb-1">
                    {profiles.length === 0 ? '还没有实例' : '未找到匹配的实例'}
                  </h3>
                  <p className="text-sm text-[var(--color-text-muted)] mb-4">
                    {profiles.length === 0
                      ? '创建第一个浏览器实例开始管理多账号环境'
                      : '尝试调整搜索或筛选条件'}
                  </p>
                  {profiles.length === 0 ? (
                    <Link to="/browser/create">
                      <Button>
                        <Plus className="w-4 h-4" />
                        新建实例
                      </Button>
                    </Link>
                  ) : (
                    <Button
                      variant="secondary"
                      onClick={() => { setSearchQuery(''); setFilters(EMPTY_FILTERS) }}
                    >
                      <XCircle className="w-4 h-4" />
                      清空筛选
                    </Button>
                  )}
                </div>
              </div>
            </div>
          ) : viewMode === 'table' ? (
            <div className="bg-[var(--color-bg-surface)] rounded-xl border border-[var(--color-border-default)] overflow-hidden">
              <Table
                columns={enhancedColumns}
                data={filteredProfiles}
                rowKey="profileId"
                stickyHeader
                maxHeight="calc(100vh - 400px)"
              />
            </div>
          ) : (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              {filteredProfiles.map((record) => {
                const core = resolveProfileCore(record)
                const proxy = proxies.find(p => p.proxyId === record.proxyId)
                return (
                  <ProfileCard
                    key={record.profileId}
                    record={record}
                    selected={selectedIds.has(record.profileId)}
                    coreLabel={core?.coreName || getProfileCoreLabel(record)}
                    proxyName={proxy?.proxyName || record.proxyId || '-'}
                    status={getProfileStatus(record)}
                    isStarting={isProfileStarting(record.profileId)}
                    isStopping={isProfileStopping(record.profileId)}
                    isBusy={isProfileBusy(record.profileId)}
                    onToggleSelect={toggleSelect}
                    onStart={handleStart}
                    onStop={handleStop}
                    onRestart={handleRestart}
                    onKeywords={openKwModal}
                    onCopy={openCopyModal}
                    onDelete={handleDelete}
                    onRefreshCode={loadProfiles}
                  />
                )
              })}
            </div>
          )}
        </div>
      </main>
