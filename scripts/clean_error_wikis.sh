#!/bin/bash
# 清理没有任何 siteinfo 记录且上一次 Siteinfo check 状态为 error 的 wiki 记录

set -e

# Docker 容器名称
DB_CONTAINER="wikikeeper-postgres"

# 数据库连接信息
DB_USER="wikikeeper"
DB_NAME="wikikeeper"

# 默认密码（可通过环境变量覆盖）
DB_PASSWORD="${POSTGRES_PASSWORD:-wikikeeper123}"

echo "=== 清理错误状态的 Wiki 记录 ==="
echo ""

# 预览将被删除的记录
echo "预览将被删除的记录："
docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t << 'SQL'
SELECT
    id,
    url,
    wiki_name,
    status,
    last_error,
    created_at
FROM wikis
WHERE status = 'error'
AND id NOT IN (SELECT wiki_id FROM wiki_stats)
ORDER BY created_at DESC;
SQL

echo ""

# 统计将被删除的记录数
COUNT=$(docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t -c "
    SELECT COUNT(*) FROM wikis
    WHERE status = 'error'
    AND id NOT IN (SELECT wiki_id FROM wiki_stats);
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
    AND id NOT IN (SELECT wiki_id FROM wiki_stats);
"

echo "删除完成！"
