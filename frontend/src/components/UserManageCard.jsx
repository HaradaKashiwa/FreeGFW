import { useEffect, useState } from "react"
import { useGetConfigs } from "../apis/config"
import { useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { IoAddCircleOutline, IoCheckmark, IoQrCode, IoTrash, IoTrashBin, IoCopy } from "react-icons/io5"
import { QRCodeSVG } from "qrcode.react"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { useAddUsers, useDeleteUser } from "../apis/user"
import { PiSpinner } from "react-icons/pi"
import { Form } from "@/components/ui/form"
import { useGetUsers } from "../apis/user"
import { Modal } from "./Modal"
import { useLanguageStore } from "../store/useLanguageStore"

export function UserManageCard() {
    const { trigger: addUsers, loading: addUsersLoading, error: addUsersError } = useAddUsers()
    const [error, setError] = useState(null)
    const { data: users, loading: usersLoading, loaded: usersLoaded, refresh: refreshUsers } = useGetUsers()
    const [open, setOpen] = useState(false)
    const [preDeleteUser, setPreDeleteUser] = useState(null)
    const [qrCodeUser, setQrCodeUser] = useState(null)
    const { trigger: deleteUser, loading: deleteUsersLoading, error: deleteUsersError } = useDeleteUser({ id: preDeleteUser?.id })
    const { t } = useLanguageStore()

    const formatBytes = (bytes, decimals = 2) => {
        if (!+bytes) return '0 B'
        const k = 1024
        const dm = decimals < 0 ? 0 : decimals
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']
        const i = Math.floor(Math.log(bytes) / Math.log(k))
        return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`
    }

    return (
        <div className='bg-white rounded-lg pt-4'>
            <div className='md:flex items-center justify-between px-4'>
                <div>
                    <div className='text-md'>{t('local_user_management')}</div>
                    <div className='text-xs opacity-50'>{t('local_user_management_desc')}</div>
                </div>
                <div className='flex items-center gap-2 mt-4 md:mt-0'>
                    <Input className='h-8' placeholder={t('search_user')} />
                    <Button className='cursor-pointer' size='sm' onClick={() => setOpen(true)}>{t('add_user')} <IoAddCircleOutline /></Button>
                </div>
            </div>
            <Modal
                open={open}
                onOpenChange={(val) => {
                    setOpen(val)
                    setError(null)
                }}
                title={t('add_new_user')}
                description={t('add_user_desc')}
                content={
                    <Form
                        onSubmit={async v => {
                            try {
                                setError(null)
                                await addUsers(v)
                                refreshUsers()
                                setOpen(false)
                            } catch (e) {
                                setError(e.message)
                            }
                        }}
                        fields={[
                            {
                                name: 'username',
                                label: t('username'),
                                component: <Input placeholder="e.g. freegfw" name="username" type="text" />,
                                description: t('username_desc')
                            }
                        ]}
                        submitLoading={addUsersLoading}
                        submitText={t('add')}
                        errors={error ? [{ field: 'username', message: error }] : []}
                    />
                }
            />
            <div className='mt-4'>
                <div className='flex items-center gap-4 p-4 py-2 font-bold border-b'>
                    <div className='flex-1 flex items-center gap-2'>
                        {t('user')}
                    </div>
                    <div className='flex-1'>
                        {t('traffic')}
                    </div>
                    <div className="flex-1 text-end pe-4">
                        {t('actions')}
                    </div>
                </div>
                <div className='max-h-96 overflow-y-auto'>
                    {!usersLoaded && <PiSpinner className='text-primary animate-spin text-2xl mx-auto m-5' />}
                    {!users?.length && usersLoaded && <div className='text-center text-sm opacity-70 m-5'>{t('no_users_yet')}</div>}
                    {users?.map(user => (
                        <div key={user.uuid} className='flex items-center gap-4 p-4 border-b last:border-b-0'>
                            <div className='flex-1 flex items-center gap-4'>
                                <img src={`https://avatar.vercel.sh/${user.username}`} className='w-8 h-8 rounded-full' />
                                <div>
                                    <div>{user.username}</div>
                                </div>
                            </div>
                            <div className='flex-1 text-xs opacity-70 flex flex-col justify-center'>
                                <div dir="ltr">↑ {formatBytes(user.upload || 0)}</div>
                                <div dir="ltr">↓ {formatBytes(user.download || 0)}</div>
                            </div>
                            <div className="flex-1 flex justify-end gap-2">
                                <Button size='sm' variant='outline' className='cursor-pointer px-0' onClick={() => setQrCodeUser(user)}><IoQrCode /> <span className='hidden md:block'>{t('connect_code')}</span></Button>
                                <Button size='sm' variant='destructive' className='cursor-pointer px-2' onClick={() => setPreDeleteUser(user)}><IoTrashBin /></Button>
                            </div>
                        </div>
                    ))}
                </div>
            </div>
            <Modal
                title={t('delete_user')}
                description={preDeleteUser ? t('delete_user_confirm', { username: preDeleteUser.username }) : ''}
                open={!!preDeleteUser}
                onOpenChange={() => setPreDeleteUser(null)}
                content={
                    <div className='flex gap-2 justify-end'>
                        <Button variant='outline' onClick={() => setPreDeleteUser(null)}>{t('cancel')}</Button>
                        <Button variant='destructive' onClick={async () => {
                            await deleteUser()
                            refreshUsers()
                            setPreDeleteUser(null)
                        }}>{t('confirm')} {deleteUsersLoading && <PiSpinner className='animate-spin' />}</Button>
                    </div>
                }
            />
            <Modal
                title={t('connection_config')}
                description={t('connection_config_desc')}
                open={!!qrCodeUser}
                onOpenChange={() => setQrCodeUser(null)}
                content={
                    <div className='space-y-4'>
                        <div className='flex justify-center p-4 bg-white rounded-lg border'>
                            {qrCodeUser && (
                                <QRCodeSVG
                                    value={`${window.location.origin}/subscribe/${qrCodeUser.uuid}`}
                                    size={200}
                                    level="H"
                                    includeMargin
                                />
                            )}
                        </div>
                        <div className="flex gap-2">
                            <Input
                                readOnly
                                value={qrCodeUser ? `${window.location.origin}/subscribe/${qrCodeUser.uuid}` : ''}
                                className="bg-gray-50 font-mono text-xs"
                            />
                            <Button variant="outline" size="icon" onClick={() => {
                                navigator.clipboard.writeText(`${window.location.origin}/subscribe/${qrCodeUser.uuid}`)
                            }}>
                                <IoCopy />
                            </Button>
                        </div>
                    </div>
                }
            />
        </div>
    )
}