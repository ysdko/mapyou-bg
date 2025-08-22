# MAPYOU
## デプロイURL
https://www.map-you.com/

## リポジトリ
- フロントエンド
  https://github.com/ysdko/mapyou-fe

- バックエンド
  https://github.com/ysdko/mapyou-bg

- スクレイピング
  https://github.com/ysdko/scraping-eventsite

## 目的
- 首都圏の遊び向けイベント情報を、ユーザーが地図上で直感的に探せるようにする。

- イベントの評価(5段階評価、口コミ)を見ることにより、イベントに行くかどうかの判断の参考にできるようにする

![img1](doc/mapyou-main.png)


## アーキテクチャ図
![img2](doc/mapyou-architecture.drawio.png)

## ER図
```mermaid
erDiagram
    USERS {
        int id PK
        string username
        string email
        datetime created_at
    }

    EVENTS {
        int id PK
        string title
        date start_date
        date end_date
        float lat
        float lng
        string location
        string site_url
        string category
        int icon_category
        int created_by FK
    }

    REVIEWS {
        int id PK
        int event_id FK
        int user_id FK
        int rating
        string comment
        datetime created_at
    }



    USERS ||--o{ REVIEWS : ""
    EVENTS ||--o{ REVIEWS : ""
```
