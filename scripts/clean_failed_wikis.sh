#!/bin/bash
# 清理失败的 wiki 记录：
# 1. 上一次 siteinfo check = error 且没有任何 siteinfo 记录
# 2. AND (api 为 null OR 运行过 archive check 但失败且没有任何 archive 记录)

set -e

# Docker 容器名称
DB_CONTAINER="wikikeeper-postgres"

# 数据库连接信息
DB_USER="wikikeeper"
DB_NAME="wikikeeper"

echo "=== 清理失败的 Wiki 记录 ==="
echo ""

# 预览将被删除的记录
echo "预览将被删除的记录："
docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t << 'SQL'
SELECT
    id,
    url,
    wiki_name,
    status,
    CASE WHEN api_url IS NULL THEN 'NULL' ELSE 'SET' END as api_status,
    last_error,
    last_error_at,
    archive_last_check_at,
    CASE WHEN archive_last_error IS NOT NULL THEN 'YES' ELSE 'NO' END as archive_failed,
    created_at
FROM wikis
WHERE status = 'error'
AND id NOT IN (SELECT wiki_id FROM wiki_stats)
AND (
    api_url IS NULL
    OR (
        archive_last_check_at IS NOT NULL
        AND archive_last_error IS NOT NULL
        AND id NOT IN (SELECT wiki_id FROM wiki_archives)
    )
)
ORDER BY created_at DESC;
SQL

echo ""

# 统计将被删除的记录数
COUNT=$(docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t -c "
    SELECT COUNT(*) FROM wikis
    WHERE status = 'error'
    AND id NOT IN (SELECT wiki_id FROM wiki_stats)
    AND (
        api_url IS NULL
        OR (
            archive_last_check_at IS NOT NULL
            AND archive_last_error IS NOT NULL
            AND id NOT IN (SELECT wiki_id FROM wiki_archives)
        )
    );
")

echo "将删除 $COUNT 条记录"
echo ""

# 确认删除
read -p "确认删除这些记录吗? (yes/no): " CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    echo "操作已取消"
    exit 0
fi

# 执行删除
echo "正在删除..."
docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c "
    DELETE FROM wikis
    WHERE status = 'error'
    AND id NOT IN (SELECT wiki_id FROM wiki_stats)
    AND (
        api_url IS NULL
        OR (
            archive_last_check_at IS NOT NULL
            AND archive_last_error IS NOT NULL
            AND id NOT IN (SELECT wiki_id FROM wiki_archives)
        )
    );
"

echo "删除完成！"
