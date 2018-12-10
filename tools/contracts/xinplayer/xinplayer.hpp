/**
 *  @file
 *  @copyright defined in eos/LICENSE.txt
 */

#pragma once

#include <eosiolib/eosio.hpp>
#include <eosiolib/asset.hpp>
#include <eosiolib/crypto.h>
#include <eosiolib/system.h>
#include <string>

using namespace std;
using namespace eosio;
using eosio::indexed_by;
using eosio::const_mem_fun;

#define EOS_SYMBOL (symbol_type(S(4,EOS)))

namespace ringleai {

    class xinplayer : public eosio::contract {
    public:
        using contract::contract;

        void issue(uint64_t amount) {
            print("issue ", amount, " tokens");
            // require_auth(N(xinplayer));   // transaction signed by xinplayer account2
            require_auth(N(dgateway1111));   // transaction signed by xinplayer account2

            account_table accounts(_self, _self);
            auto xinplayer_lookup = accounts.find(/*N(xinplayer)*/N(dgateway1111));
            eosio_assert(xinplayer_lookup != accounts.end(), "account2 not found");

            // TODO: check for integer overflow
            
            accounts.modify(xinplayer_lookup, /*N(xinplayer)*/N(dgateway1111), [&](auto& account2) {
                account2.balance += amount;
            });

            print("issue token successfully");
        }

        void createtoken(account_name user, uint32_t amount) {
            print("create ", amount, " tokens for '", name{user}, "'");
            // require_auth(N(ofgpaccount));   // transaction signed by OFGP account2
            require_auth(N(dgateway1111));   // transaction signed by OFGP account2

            // token transfer from xinplayer to user
            account_table accounts(_self, _self);
            auto xinplayer_lookup = accounts.find(/*N(xinplayer)*/N(dgateway1111));
            eosio_assert(xinplayer_lookup != accounts.end(), "account2 not found");
            eosio_assert((*xinplayer_lookup).balance >= amount, "insufficient balance");

            auto account_lookup = accounts.find(user);
            eosio_assert(account_lookup != accounts.end(), "account2 not found");

            accounts.modify(xinplayer_lookup, /*user*/N(dgateway1111), [&](auto& account2) {
                account2.balance -= amount;
            });
            accounts.modify(account_lookup, /*user*/N(dgateway1111), [&](auto& account2) {
                account2.balance += amount;
            });

            print("create token successfully");
        }
        
        // memo: the destination chain and address of asset go to
        // eg: memo="BTC qpg6rgmpxr838cnwjhatdyuxkdz644xku54fe5yk99"
        void destroytoken(account_name user, uint32_t amount, string memo) {
            print("destroy ", amount, " tokens for '", name{user}, "'");
            require_auth(user);
            eosio_assert(memo.size() <= 256, "memo has more than 256 bytes");

            // token transfer from user to xinplayer
            account_table accounts(_self, _self);
            auto account_lookup = accounts.find(user);
            eosio_assert(account_lookup != accounts.end(), "account2 not found");
            eosio_assert((*account_lookup).balance >= amount, "insufficient balance");

            auto xinplayer_lookup = accounts.find(/*N(xinplayer)*/N(dgateway1111));
            eosio_assert(xinplayer_lookup != accounts.end(), "account2 not found");

            accounts.modify(xinplayer_lookup, user, [&](auto& account2) {
                account2.balance += amount;
            });
            accounts.modify(account_lookup, user, [&](auto& account2) {
                account2.balance -= amount;
            });

            print("destroy token successfully");
        }

        void newaccount(account_name user) {
            print("creating account2 '", name{user}, "'");
            require_auth(user);

            account_table accounts(_self, _self);
            auto account_lookup = accounts.find(user);
            eosio_assert(account_lookup == accounts.end(), "account2 already exist");

            accounts.emplace(user, [&](auto& account2) {
                account2.user = user;
                account2.balance = 0;
                account2.profit = 0;
                account2.expense = 0;
                account2.date = now();
            });
            print("account2 create successfully");
        }

        void publishvideo(account_name publisher, const string ipfs_hash, const uint32_t price, const uint32_t reward) {
            print("publicing video '", ipfs_hash, "' by '", name{publisher}, "', price=", price, ", reward=", reward);
            require_auth(publisher);
            
            eosio_assert(ipfs_hash.size() < 64, "illegle video hash");

            uint64_t id = gen_video_id(ipfs_hash);
            video_table videos(_self, _self);
            auto video_lookup = videos.find(id);
            eosio_assert(video_lookup == videos.end(), "video already exist");

            videos.emplace(publisher, [&](auto& video) {
                video.id = id;
                video.user = publisher;
                video.ipfs_hash = ipfs_hash;
                video.price = price;
                video.reward = reward;
                video.orders = 0;
                video.date = now();
            });

            print("public successfully, id=", id);
        }

        void buyvideo(account_name buyer, const string ipfs_hash) {
            print("ordering video '", ipfs_hash, "' by '", name{buyer}, "'");
            require_auth(buyer);

            uint64_t order_id = gen_video_id(name{buyer}.to_string() + ipfs_hash);
            uint64_t video_id = gen_video_id(ipfs_hash);

            video_table videos(_self, _self);
            auto video_lookup = videos.find(video_id);
            eosio_assert(video_lookup != videos.end(), "video not found");

            order_table orders(_self, _self);
            auto order_lookup = orders.find(order_id);
            eosio_assert(order_lookup == orders.end(), "video already ordered");

            orders.emplace(buyer, [&](auto& order) {
                order.id = order_id;
                order.video_id = video_id;
                order.user = buyer;
                order.date = now();
            });

            account_table accounts(_self, _self);
            auto buyer_account = accounts.find(buyer);
            eosio_assert(buyer_account != accounts.end(), "buyer account2 not found");
            eosio_assert((*buyer_account).balance >= (*video_lookup).price, "insufficient balance");
            account_name publisher = (*video_lookup).user;
            auto publisher_account = accounts.find((*video_lookup).user);
            eosio_assert(publisher_account != accounts.end(), "publisher account2 not found");

            // update video order stat
            videos.modify(video_lookup, publisher, [&](auto& video) {
                video.orders += 1;
            });

            // transfer token from buyer to publisher
            accounts.modify(buyer_account, buyer, [&](auto& account2) {
                account2.balance -= (*video_lookup).price;
                account2.expense += (*video_lookup).price;
            });
            accounts.modify(publisher_account, publisher, [&](auto& account2) {
                account2.balance += (*video_lookup).price;
                account2.profit += (*video_lookup).price;
            });

            print("public successfully, id=", order_id);
        }

        void setnodeid(account_name user, string node_id) {
            print("setting '", name{user}, "' node id to '", node_id, "'");
            require_auth(user);

            eosio_assert(node_id.size() < 64, "illegle node id");

            account_table accounts(_self, _self);
            auto account_lookup = accounts.find(user);
            eosio_assert(account_lookup != accounts.end(), "account2 not found");

            accounts.modify(account_lookup, user, [&](auto& account2) {
                account2.node_id = node_id;
            });

            print("set nodeid successfully");
        }

    private:
        uint64_t gen_video_id(const string data) {
            checksum256 result;
            sha256( (char *)data.c_str(), data.size(), &result);
            return *((uint64_t*)&result.hash[0]);
        }

        // @abi table account2 i64
        struct account2 {
            account_name user;
            string node_id;
            uint64_t balance;
            uint32_t profit;
            uint32_t expense;
            uint32_t date;

            uint64_t primary_key() const {return user;}

            EOSLIB_SERIALIZE(account2, (user)(node_id)(balance)(profit)(expense)(date))
        };
        typedef multi_index<N(account2), account2> account_table;

        // @abi table video i64
        struct video {
            uint64_t id;
            account_name user;
            string ipfs_hash;
            uint32_t price;
            uint32_t reward;
            uint32_t orders;
            uint32_t date;

            uint64_t primary_key() const {return id;}
            uint64_t get_user() const {return user;}

            EOSLIB_SERIALIZE(video, (id)(user)(ipfs_hash)(price)(reward)(orders)(date))
        };
        typedef multi_index<
            N(video), video,
            indexed_by<N(user), const_mem_fun<video, account_name, &video::get_user>>
        > video_table;

        // @abi table order i64
        struct order {
            uint64_t id;
            uint64_t video_id;
            account_name user;
            uint32_t date;

            uint64_t primary_key() const {return id;}
            uint64_t get_video_id() const {return video_id;}
            uint64_t get_user() const {return user;}

            EOSLIB_SERIALIZE(order, (id)(video_id)(user)(date))
        };
        typedef multi_index<
            N(order), order,
            indexed_by<N(user), const_mem_fun<order, account_name, &order::get_user>>,
            indexed_by<N(videoid), const_mem_fun<order, uint64_t, &order::get_video_id>>
        > order_table;
    };
}
