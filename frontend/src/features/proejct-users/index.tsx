import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Main } from '@/components/layout/main';
import { Header } from '@/components/layout/header';
import { createColumns } from './components/users-columns';
import { UsersDialogs } from './components/users-dialogs';
import { UsersPrimaryButtons } from './components/users-primary-buttons';
import { UsersTable } from './components/users-table';
import UsersProvider from './context/users-context';
import { useUsers } from './data/users';

function UsersContent() {
  const { t } = useTranslation();
  const { userPermissions, rolePermissions } = usePermissions();
  const { pageSize, setCursors, setPageSize, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'project-users-table-page-size',
  });

  // Memoize columns to prevent infinite re-renders
  const columns = useMemo(
    () => createColumns(t, userPermissions.canWrite, rolePermissions.canRead),
    [t, userPermissions.canWrite, rolePermissions.canRead]
  );

  const {
    data,
    isLoading,
    error: _error,
  } = useUsers({
    ...paginationArgs,
  });

  const [nameFilter, setNameFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [roleFilter, setRoleFilter] = useState<string[]>([]);
  const debouncedNameFilter = useDebounce(nameFilter, 300);
  // Project.projectUsers returns the complete membership list without filter
  // arguments. Filter that list here while preserving project-specific scopes.
  const projectUsers = useMemo(() => {
    const search = debouncedNameFilter.trim().toLowerCase();
    return (data?.edges?.map((edge) => edge.node) || []).filter((user) =>
      (!search || `${user.firstName} ${user.lastName} ${user.email}`.toLowerCase().includes(search)) &&
      (statusFilter.length === 0 || statusFilter.includes(user.status)) &&
      (roleFilter.length === 0 || user.roles?.edges?.some(({ node }) => roleFilter.includes(node.id)))
    );
  }, [data, debouncedNameFilter, statusFilter, roleFilter]);
  const pageInfo = data?.pageInfo;

  const handleNextPage = () => {
    if (data?.pageInfo?.hasNextPage && data?.pageInfo?.endCursor) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'after');
    }
  };

  const handlePreviousPage = () => {
    if (data?.pageInfo?.hasPreviousPage) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'before');
    }
  };

  const handlePageSizeChange = (nextPageSize: number) => {
    setPageSize(nextPageSize);
  };

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <UsersTable
        data={projectUsers}
        columns={columns}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        nameFilter={nameFilter}
        statusFilter={statusFilter}
        roleFilter={roleFilter}
        onNameFilterChange={setNameFilter}
        onStatusFilterChange={setStatusFilter}
        onRoleFilterChange={setRoleFilter}
      />
    </div>
  );
}

export default function UsersManagement() {
  const { t } = useTranslation();

  return (
    <UsersProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-lg font-normal tracking-tight'>{t('projectUsers.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('projectUsers.description')}</p>
          </div>
          <UsersPrimaryButtons />
        </div>
      </Header>

      <Main fixed>
        <UsersContent />
      </Main>
      <UsersDialogs />
    </UsersProvider>
  );
}
